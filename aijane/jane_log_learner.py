"""
jane_log_learner.py — learn properties of the Jane Attestation Server's log
stream in near real time, using a local Ollama model.

Input
  Jane writes every log entry to MongoDB, to its CSV log file, and to MQTT on
  topic AS/<channel> (SYS, IM, S, C, R). The file and MQTT payloads share one
  line format (logging.makeCSVText):

      itemid,timestamp_ns,channel,operation,refid,reftype,"message"

  This script reads either source:
      python jane_log_learner.py mqtt --broker 192.168.1.203
      python jane_log_learner.py file /var/log/jane.log [--from-start] [--once]

Approach
  1. Code parses each entry, turns it into an event type such as S/open/session
     or S/update/session:claim, and keeps a trace, per-session lifecycles
     (open -> claim* -> result* -> close), a transition model, and result-code
     counts. It flags new event types, never-seen transitions, orphan session
     updates, stale sessions and failure bursts as they happen.
  2. Every --window seconds the model gets a compact summary and proposes
     temporal properties from a fixed vocabulary: absence, precedes, response
     (within a time bound), count_relation, rate_max and result_fraction.
  3. Code checks each proposal against the trace and reports support
     (instances checked) and confidence (share satisfied), the way
     specification miners do. Proposals refuted for 3 windows are dropped.

  For session-scoped properties Jane needs Logging.SessionUpdateLogging: true;
  otherwise claims and results can't be tied to their sessions and only
  global-scope properties are meaningful.

pip install ollama paho-mqtt
ollama pull qwen2.5:7b
"""

import argparse, ast, json, statistics, threading, time
from collections import Counter, deque

RESULT_CODES = {0: "Success", 9001: "Fail", 9010: "VerifyCallFailure",
                9098: "VerifyClaimErrorAttempt", 9099: "NoResult",
                9997: "MissingExpectedValue", 9998: "RuleCallFailure",
                9999: "UnsetResultValue"}
SESSION_EVENTS = ["open", "claim", "result", "close"]


# ---------------------------------------------------------------- parsing
def parse(line):
    """Parse one Jane CSV log line. The message is Go %q-quoted."""
    parts = line.strip().split(",", 6)
    if len(parts) < 7:
        return None
    itemid, ts, ch, op, ref, rtype, msg = parts
    if len(msg) >= 2 and msg[0] == msg[-1] == '"':
        try:
            msg = ast.literal_eval(msg)      # safe: literals only
        except (ValueError, SyntaxError):
            msg = msg[1:-1]
    try:
        t = int(ts) / 1e9
    except ValueError:
        t = time.time()
    return {"itemid": itemid, "t": t, "channel": ch, "op": op,
            "refid": ref, "reftype": rtype, "msg": msg, "raw": line.strip()}


def event_type(e):
    rtype = e["reftype"]
    if e["channel"] == "SYS":            # reftype carries names/versions
        rtype = rtype.split()[0] if rtype else ""
    k = f"{e['channel']}/{e['op']}/{rtype}"
    if e["channel"] == "S" and e["op"] == "update":
        k += ":" + e["msg"].split(",", 1)[0]
    return k


# ---------------------------------------------------------------- state
class State:
    def __init__(self, maxlen=20000, stale_s=300):
        self.lock = threading.Lock()
        self.trace = deque(maxlen=maxlen)   # (t, event_type, entry)
        self.counts = Counter()
        self.trans = Counter()
        self.prev = None
        self.sessions = {}                  # sid -> {"open", "close", "events"}
        self.codes = {}                     # result id -> code
        self.anomalies = deque(maxlen=50)
        self.stale_s = stale_s
        self.last_t = 0.0

    def flag(self, t, kind, detail):
        self.anomalies.append({"t": round(t, 3), "kind": kind, "detail": detail})

    def ingest(self, e):
        k, t = event_type(e), e["t"]
        with self.lock:
            n = sum(self.counts.values())
            if n > 50 and k not in self.counts:
                self.flag(t, "new_event_type", k)
            if self.prev and n > 200 and self.trans[(self.prev, k)] == 0:
                self.flag(t, "unseen_transition", f"{self.prev} -> {k}")
            self.counts[k] += 1
            if self.prev:
                self.trans[(self.prev, k)] += 1
            self.prev, self.last_t = k, max(self.last_t, t)
            self.trace.append((t, k, e))

            ch, op, ref = e["channel"], e["op"], e["refid"]
            if ch == "S":
                s = self.sessions.get(ref)
                if op == "open":
                    self.sessions[ref] = {"open": t, "close": None,
                                          "events": [(t, "open", None)], "msg": e["msg"]}
                elif s is None:
                    self.flag(t, f"{op}_for_unknown_session", ref)
                elif op == "update":
                    kind, _, rid = e["msg"].partition(",")
                    s["events"].append((t, kind, rid))
                elif op == "close":
                    if s["close"] is not None:
                        self.flag(t, "double_close", ref)
                    s["close"] = t
                    s["events"].append((t, "close", None))
                if len(self.sessions) > 5000:
                    for sid in [x for x, v in self.sessions.items() if v["close"]][:1000]:
                        del self.sessions[sid]
            elif ch == "R" and op == "add":
                try:
                    code = int(e["msg"])
                except ValueError:
                    self.flag(t, "unparseable_result", e["msg"])
                    return
                self.codes[ref] = code
                if code not in RESULT_CODES:
                    self.flag(t, "unknown_result_code", code)
                recent = [c for (_, kk, ee) in list(self.trace)[-20:] if kk == "R/add/result"
                          for c in [int(ee["msg"]) if ee["msg"].lstrip("-").isdigit() else 0]]
                if len(recent) >= 10 and sum(c != 0 for c in recent) / len(recent) > 0.5:
                    self.flag(t, "failure_burst", f"{sum(c != 0 for c in recent)}/{len(recent)} recent results non-zero")

    # sequences for checking --------------------------------------------
    def session_seqs(self, closed_only=False):
        return [[(t, kind) for t, kind, _ in s["events"]]
                for s in self.sessions.values()
                if s["open"] is not None and (s["close"] or not closed_only)]

    def global_seq(self):
        return [(t, k) for t, k, _ in self.trace]

    def result_codes(self):
        out = []
        for _, k, e in self.trace:
            if k == "R/add/result":
                try:
                    out.append(int(e["msg"]))
                except ValueError:
                    pass
        return out

    # summary for the model ----------------------------------------------
    def summary(self, n_raw=8):
        with self.lock:
            seq = list(self.trace)
            span = (seq[-1][0] - seq[0][0]) / 60 if len(seq) > 1 else 0
            sess = [s for s in self.sessions.values() if s["open"] is not None]
            closed = [s for s in sess if s["close"]]
            stale = [sid for sid, s in self.sessions.items() if s["open"] and not s["close"]
                     and self.last_t - s["open"] > self.stale_s]
            per = lambda kind: Counter(sum(1 for _, k, _ in s["events"] if k == kind) for s in closed)
            durs = [s["close"] - s["open"] for s in closed]
            codes = Counter(RESULT_CODES.get(c, str(c)) for c in self.result_codes())
            return {
                "window_minutes": round(span, 1),
                "event_counts": dict(self.counts.most_common(30)),
                "top_transitions": {f"{a} -> {b}": n for (a, b), n in self.trans.most_common(30)},
                "sessions": {
                    "tracked": len(sess), "closed": len(closed),
                    "stale_open_over_s": {"threshold": self.stale_s, "count": len(stale)},
                    "claims_per_closed_session": dict(per("claim")),
                    "results_per_closed_session": dict(per("result")),
                    "duration_s": ({"median": round(statistics.median(durs), 3),
                                    "max": round(max(durs), 3)} if durs else None),
                    "session_update_logging_seen": any(k.startswith("S/update") for k in self.counts),
                },
                "result_codes": dict(codes),
                "recent_anomalies": list(self.anomalies)[-15:],
                "recent_lines": [e["raw"] for _, _, e in seq[-n_raw:]],
            }


# ---------------------------------------------------------------- checking
def check(p, st):
    """Return (satisfied, violated) instance counts for a proposed property."""
    kind, a, b = p.get("kind"), p.get("a"), p.get("b")
    session = p.get("scope") == "session"
    with st.lock:
        if kind == "count_relation":
            seqs = st.session_seqs(closed_only=True)
        elif kind in ("rate_max", "result_fraction"):
            seqs = None
        else:
            seqs = st.session_seqs() if session else [st.global_seq()]
        now = st.last_t
        codes = st.result_codes() if kind == "result_fraction" else None
        gseq = st.global_seq() if kind == "rate_max" else None

    sat = viol = 0
    if kind == "absence":
        for seq in seqs:
            if any(x == a for _, x in seq): viol += 1
            else: sat += 1
    elif kind == "precedes":                 # no b before the first a
        for seq in seqs:
            first_b = next((i for i, (_, x) in enumerate(seq) if x == b), None)
            if first_b is None:
                continue
            if any(x == a for _, x in seq[:first_b]): sat += 1
            else: viol += 1
    elif kind == "response":                 # every a followed by b within W s
        w = float(p.get("within_s") or 60)
        for seq in seqs:
            for i, (t, x) in enumerate(seq):
                if x != a or now - t < w:    # too recent to judge yet
                    continue
                if any(y == b and t2 - t <= w for t2, y in seq[i + 1:]): sat += 1
                else: viol += 1
    elif kind == "count_relation":           # per closed session
        rel = p.get("relation", "eq")
        for seq in seqs:
            na, nb = sum(x == a for _, x in seq), sum(x == b for _, x in seq)
            ok = {"eq": na == nb, "le": na <= nb, "ge": na >= nb}.get(rel)
            if ok is None: return 0, 0
            if ok: sat += 1
            else: viol += 1
    elif kind == "rate_max":                 # per-minute buckets
        bound = float(p.get("per_minute") or 0)
        buckets = Counter(int(t // 60) for t, x in gseq if x == a)
        for m in range(min(buckets, default=0), max(buckets, default=-1) + 1):
            if buckets[m] <= bound: sat += 1
            else: viol += 1
    elif kind == "result_fraction":
        if len(codes) >= 10:
            frac = sum(c == p.get("code") for c in codes) / len(codes)
            ok = frac <= p["p"] if p.get("op") == "le" else frac >= p["p"]
            sat, viol = (1, 0) if ok else (0, 1)
    return sat, viol


def status(sat, viol, min_support=5):
    n = sat + viol
    if n < min_support: return "insufficient"
    if viol == 0: return "holds"
    return "likely" if sat / n >= 0.95 else "refuted"


# ---------------------------------------------------------------- the model
SCHEMA = {
    "type": "object",
    "properties": {
        "properties": {"type": "array", "items": {"type": "object", "properties": {
            "kind": {"enum": ["absence", "precedes", "response", "count_relation",
                              "rate_max", "result_fraction"]},
            "scope": {"enum": ["session", "global"]},
            "a": {"type": "string"}, "b": {"type": "string"},
            "within_s": {"type": "number"},
            "relation": {"enum": ["eq", "le", "ge"]},
            "per_minute": {"type": "number"},
            "code": {"type": "integer"}, "op": {"enum": ["le", "ge"]}, "p": {"type": "number"},
            "rationale": {"type": "string"}},
            "required": ["kind", "scope", "a", "rationale"]}},
        "observations": {"type": "string"},
        "anomaly_assessment": {"type": "string"},
    },
    "required": ["properties", "observations", "anomaly_assessment"],
}

SYSTEM = f"""You analyse the live audit log of Jane, a remote attestation server.
Channels: SYS system start/stop; IM information model changes (elements,
intents, expected values, rules, protocols, opaque objects); S attestation
sessions; C claims (evidence collected from an element); R verification results.
A session normally runs open -> claim* -> result* -> close.
Result codes: {json.dumps(RESULT_CODES)}.

Propose temporal properties that the stream appears to satisfy and that would
be useful to monitor for operational or security reasons.
Event names: with scope "session" use only {SESSION_EVENTS};
with scope "global" use event types exactly as in event_counts.
Kinds:
  absence(a)                 a never occurs
  precedes(a,b)              no b before the first a
  response(a,b,within_s)     every a is followed by b within within_s seconds
  count_relation(a,b,relation) per closed session, count(a) eq/le/ge count(b)
  rate_max(a,per_minute)     a occurs at most per_minute times in any minute (global)
  result_fraction(code,op,p) share of results with this code is le/ge p (a="result", global)
Keep hypotheses that hold, revise refuted ones, avoid duplicates, propose at most 6.
In anomaly_assessment, say whether recent anomalies look benign, operational or
security-relevant (e.g. expected values changed just before results flip)."""


def learn_round(st, hyps, model, out_path=None):
    import ollama
    report = []
    for h in hyps.values():
        report.append({**h["prop"], "status": h["status"],
                       "support": h["sat"] + h["viol"], "violations": h["viol"]})
    prompt = json.dumps({**st.summary(), "current_hypotheses": report}, default=str)
    try:
        resp = ollama.chat(model=model, format=SCHEMA, options={"temperature": 0.2},
                           messages=[{"role": "system", "content": SYSTEM},
                                     {"role": "user", "content": prompt}])
        out = json.loads(resp["message"]["content"])
    except Exception as ex:
        print("ollama error:", ex)
        return

    for p in out.get("properties", []):
        key = json.dumps({k: v for k, v in p.items() if k != "rationale"}, sort_keys=True)
        hyps.setdefault(key, {"prop": p, "refuted_rounds": 0})
    for key, h in list(hyps.items()):
        h["sat"], h["viol"] = check(h["prop"], st)
        single = h["prop"].get("kind") == "result_fraction" or (
            h["prop"].get("kind") == "absence" and h["prop"].get("scope") != "session")
        h["status"] = status(h["sat"], h["viol"], min_support=1 if single else 5)
        h["refuted_rounds"] = h["refuted_rounds"] + 1 if h["status"] == "refuted" else 0
        if h["refuted_rounds"] >= 3:
            del hyps[key]

    print(f"\n=== {time.strftime('%H:%M:%S')} | {len(st.trace)} entries ===")
    print("observations:", out.get("observations", ""))
    print("anomalies:   ", out.get("anomaly_assessment", ""))
    order = {"holds": 0, "likely": 1, "insufficient": 2, "refuted": 3}
    for h in sorted(hyps.values(), key=lambda h: (order[h["status"]], -h["sat"])):
        print("  " + fmt(h))
    if out_path:
        with open(out_path, "w") as f:
            json.dump([{**h["prop"], "status": h["status"], "satisfied": h["sat"],
                        "violated": h["viol"]} for h in hyps.values()], f, indent=2)


def fmt(h):
    p = h["prop"]
    args = ", ".join(str(p[k]) for k in ("a", "b", "within_s", "relation", "per_minute", "code", "op", "p")
                     if p.get(k) not in (None, ""))
    return (f"[{h['status']:12} {h['sat']}/{h['sat'] + h['viol']}] "
            f"{p['kind']}[{p.get('scope', 'global')}]({args}) — {p.get('rationale', '')}")


# ---------------------------------------------------------------- sources
def run_mqtt(st, args):
    import paho.mqtt.client as mqtt

    def on_connect(c, _u, _f, rc, _p):
        print(f"connected ({rc}), subscribing to {args.topic}")
        c.subscribe(args.topic)

    def on_message(_c, _u, msg):
        e = parse(msg.payload.decode(errors="replace"))
        if e: st.ingest(e)

    c = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    c.on_connect, c.on_message = on_connect, on_message
    c.connect(args.broker, args.port)
    c.loop_forever()


def run_file(st, args):
    with open(args.path, errors="replace") as f:
        if not args.from_start:
            f.seek(0, 2)
        buf = ""
        while True:
            chunk = f.readline()
            if not chunk:
                if args.once:
                    return
                time.sleep(0.5)
                continue
            buf += chunk
            if buf.endswith("\n"):
                e = parse(buf)
                if e: st.ingest(e)
                buf = ""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--model", default="qwen2.5:7b")
    ap.add_argument("--window", type=int, default=60, help="seconds between model rounds")
    ap.add_argument("--stale", type=int, default=300, help="seconds before an open session counts as stale")
    ap.add_argument("--out", default="jane_properties.json")
    sub = ap.add_subparsers(dest="source", required=True)
    m = sub.add_parser("mqtt")
    m.add_argument("--broker", default="localhost")
    m.add_argument("--port", type=int, default=1883)
    m.add_argument("--topic", default="AS/#")
    f = sub.add_parser("file")
    f.add_argument("path")
    f.add_argument("--from-start", action="store_true", help="replay the existing file first")
    f.add_argument("--once", action="store_true", help="read the file, run one round, exit")
    args = ap.parse_args()

    st, hyps = State(stale_s=args.stale), {}
    if args.source == "file" and args.once:
        args.from_start = True
        run_file(st, args)
        learn_round(st, hyps, args.model, args.out)
        return

    def loop():
        while True:
            time.sleep(args.window)
            if st.trace:
                learn_round(st, hyps, args.model, args.out)
    threading.Thread(target=loop, daemon=True).start()
    (run_mqtt if args.source == "mqtt" else run_file)(st, args)


if __name__ == "__main__":
    main()
