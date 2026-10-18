"""
jane_situation.py — turn the Jane Attestation Server log stream into a plain-
language situation report, and give the model that text instead of raw data.

Pipeline:  log entries -> State (parsing, sessions, anomalies; from
jane_log_learner.py) -> Narrator (deterministic text) -> Ollama (assessment and
property proposals) -> code checks the proposals -> results go into the next
report.

The report text is produced by templates, not by a model, so every sentence is
traceable to log entries and the same input gives the same text. The model only
interprets.

Needs jane_log_learner.py in the same folder.
    python jane_situation.py --no-llm file jane.log --once      # just print the report
    python jane_situation.py file jane.log --from-start
    python jane_situation.py mqtt --broker 192.168.1.203
"""

import argparse, json, statistics, threading, time
from collections import Counter, deque

import jane_log_learner as J

OPS = {"add": "added", "update": "updated", "delete": "deleted"}
NOUNS = {"expectedvalue": "expected value", "stdintent": "standard intent",
         "element": "element", "intent": "intent", "rule": "rule",
         "protocol": "protocol", "object": "opaque object"}
EVENT_TEXT = {"S/open/session": "a session opening", "S/close/session": "a session closing",
              "S/update/session:claim": "a claim being added to a session",
              "S/update/session:result": "a result being added to a session",
              "C/add/claim": "a claim being stored", "R/add/result": "a verification result",
              "open": "the session opening", "close": "the session closing",
              "claim": "a claim", "result": "a result"}


def clock(t):
    return time.strftime("%H:%M:%S", time.localtime(t))


def ago(seconds):
    s = int(seconds)
    if s < 90: return f"{s} seconds"
    if s < 5400: return f"{round(s / 60)} minutes"
    return f"{s / 3600:.1f} hours"


def event_text(k):
    if k in EVENT_TEXT: return EVENT_TEXT[k]
    ch, op, rest = (k.split("/") + ["", ""])[:3]
    if ch == "IM":
        noun = NOUNS.get(rest, rest)
        return f"{'an' if noun[:1] in 'aeiou' else 'a'} {noun} being {OPS.get(op, op)}"
    if ch == "SYS": return f"a system {op} of {rest}"
    return k


def plural(n, word, many=None):
    return f"{n} {word if n == 1 else (many or word + 's')}"


def rel_level(x, hist):
    """Describe x relative to earlier windows."""
    if len(hist) < 3: return ""
    m, sd = statistics.mean(hist), statistics.pstdev(hist)
    if m == 0 and x == 0: return " This is in line with earlier periods."
    if x > m + max(2 * sd, 0.5 * m): return f" This is higher than usual (typically about {m:.0f})."
    if x < m - max(2 * sd, 0.5 * m): return f" This is lower than usual (typically about {m:.0f})."
    return f" This is in line with earlier periods (typically about {m:.0f})."


def prop_text(p):
    k, a, b = p.get("kind"), p.get("a", ""), p.get("b", "")
    where = " in every session" if p.get("scope") == "session" else ""
    if k == "absence": return f"{event_text(a)} never happens{where}"
    if k == "precedes": return f"{event_text(b)} never comes before {event_text(a)}{where}"
    if k == "response":
        return f"{event_text(a)} is always followed by {event_text(b)} within {p.get('within_s', 60):g} seconds{where}"
    if k == "count_relation":
        rel = p.get("relation")
        if rel == "le": return f"each closed session has no more {a}s than {b}s"
        if rel == "ge": return f"each closed session has at least as many {a}s as {b}s"
        return f"each closed session has the same number of {a}s and {b}s"
    if k == "rate_max": return f"{event_text(a)} happens at most {p.get('per_minute', 0):g} times a minute"
    if k == "result_fraction":
        name = J.RESULT_CODES.get(p.get("code"), str(p.get("code")))
        return f"at {'least' if p.get('op') == 'ge' else 'most'} {p.get('p', 0) * 100:.0f}% of results are {name}"
    return json.dumps(p)


class Narrator:
    def __init__(self, window_s=300):
        self.window_s = window_s
        self.history = {"opened": deque(maxlen=48), "results": deque(maxlen=48), "fail_rate": deque(maxlen=48)}

    def describe(self, st, hyps=None, window_s=None):
        w = window_s or self.window_s
        with st.lock:
            trace = list(st.trace)
            sessions = dict(st.sessions)
            codes = dict(st.codes)
            anomalies = list(st.anomalies)
            now = st.last_t
            stale_s = st.stale_s
        if not trace:
            return "No log entries have been received yet."
        start = now - w
        win = [(t, k, e) for t, k, e in trace if t >= start]
        rid2sid = {ref: sid for sid, s in sessions.items() for _, kind, ref in s["events"] if kind == "result"}
        label = lambda sid: (f"session \"{sessions[sid]['msg']}\"" if sid in sessions and sessions[sid].get("msg")
                             else f"session {sid[:8]}")
        out = []

        # header and liveness
        out.append(f"Situation report at {clock(now)}, covering the last {ago(w)} "
                   f"({plural(len(win), 'log entry', 'log entries')}).")
        if not win:
            out.append(f"The server has been silent: the last log entry was {ago(now - trace[-1][0])} ago.")

        # system lifecycle
        sys_events = [(t, k, e) for t, k, e in trace if e["channel"] == "SYS"]
        if sys_events:
            t, k, e = sys_events[-1]
            what = "started" if e["op"].startswith("startup") else "shut down"
            out.append(f"The server last {what} at {clock(t)}, {ago(now - t)} ago.")
            restarts = sum(1 for t, _, e in win if e["op"] == "startup/INIT")
            if restarts > 1:
                out.append(f"It restarted {restarts} times in this period.")
        else:
            out.append("No startup or shutdown appears in the observed log.")

        # activity
        opened = [e["refid"] for t, k, e in win if k == "S/open/session"]
        closed = [e["refid"] for t, k, e in win if k == "S/close/session"]
        out.append(f"{plural(len(opened), 'attestation session').capitalize()} opened and "
                   f"{len(closed)} closed." + rel_level(len(opened), self.history["opened"]))

        # session shape
        shape = Counter()
        slow, durs = [], []
        for sid in closed:
            s = sessions.get(sid)
            if not s or s["open"] is None:
                shape["closed without a logged opening"] += 1
                continue
            kinds = [k for _, k, _ in s["events"]]
            nc, nr = kinds.count("claim"), kinds.count("result")
            durs.append((s["close"] - s["open"], sid))
            if nc == 0 and any(k.startswith("S/update") for _, k, _ in trace): shape["closed with no claims"] += 1
            elif nr < nc: shape["closed with fewer results than claims"] += 1
            elif nr > nc: shape["closed with more results than claims"] += 1
            else: shape["normal"] += 1
        if closed:
            parts = [f"{n} {why}" for why, n in shape.items() if why != "normal"]
            if not parts:
                out.append("Every closed session followed the usual pattern of open, claims, matching results, close.")
            else:
                out.append(f"{shape['normal']} closed sessions followed the usual pattern; " + "; ".join(parts) + ".")
        if durs:
            med = statistics.median(d for d, _ in durs)
            out.append(f"Sessions took a median of {med:.1f} seconds.")
            slow = [(d, sid) for d, sid in durs if d > max(3 * med, med + 5)]
            if slow:
                out.append("Unusually slow: " + ", ".join(f"{label(sid)} ({d:.1f} s)" for d, sid in slow[:3]) + ".")
        stale = [(sid, s) for sid, s in sessions.items() if s["open"] and not s["close"] and now - s["open"] > stale_s]
        if stale:
            sid, s = min(stale, key=lambda x: x[1]["open"])
            out.append(f"{plural(len(stale), 'session')} {'has' if len(stale) == 1 else 'have'} been open for over "
                       f"{ago(stale_s)} without closing; the oldest is {label(sid)}, opened at {clock(s['open'])}.")

        # verification results
        res = [(t, e) for t, k, e in win if k == "R/add/result"]
        rcodes = [(t, codes.get(e["refid"]), e["refid"]) for t, e in res]
        fails = [(t, c, rid) for t, c, rid in rcodes if c not in (0, None)]
        rate = len(fails) / len(rcodes) if rcodes else 0.0
        if rcodes:
            msg = f"{plural(len(rcodes), 'verification result')} were recorded; {len(rcodes) - len(fails)} succeeded"
            if fails:
                by = Counter(J.RESULT_CODES.get(c, str(c)) for _, c, _ in fails)
                msg += f" and {len(fails)} did not (" + ", ".join(f"{n} {name}" for name, n in by.most_common()) + ")"
            hist = list(self.history["fail_rate"])
            if len(hist) >= 3:
                m = statistics.mean(hist)
                if rate > m + 0.15: msg += f". That failure rate ({rate:.0%}) is higher than usual (about {m:.0%})"
                elif rate < m - 0.15: msg += f". That failure rate ({rate:.0%}) is lower than usual (about {m:.0%})"
            out.append(msg + ".")
            where = Counter(label(rid2sid[rid]) for _, _, rid in fails if rid in rid2sid)
            if where:
                out.append("Failures occurred in " + ", ".join(f"{s} ({n})" if n > 1 else s
                                                               for s, n in where.most_common(4)) + ".")
            if len(fails) >= 3:
                out.append(f"The first failure was at {clock(fails[0][0])} and the most recent at {clock(fails[-1][0])}.")
        else:
            out.append("No verification results were recorded.")

        # information model changes and what followed
        changes = [(t, k, e) for t, k, e in win if e["channel"] == "IM"]
        if changes:
            desc = [f"{NOUNS.get(e['reftype'], e['reftype'])} {e['refid'][:16] or '(unnamed)'} "
                    f"{OPS.get(e['op'], e['op'])} at {clock(t)}" for t, k, e in changes[:6]]
            out.append(f"The information model changed {plural(len(changes), 'time')}: " + "; ".join(desc)
                       + ("; and more" if len(changes) > 6 else "") + ".")
            all_res = [(t, codes.get(e["refid"])) for t, k, e in trace if k == "R/add/result"]
            for tc, k, e in changes:
                what = f"{NOUNS.get(e['reftype'], e['reftype'])} {e['refid'][:16]} was {OPS.get(e['op'], e['op'])}"
                before = [c for t, c in all_res if tc - 300 <= t < tc]
                after = [c for t, c in all_res if tc < t <= tc + 300]
                if len(after) >= 5:
                    fb = sum(c != 0 for c in before) / len(before) if before else 0
                    fa = sum(c != 0 for c in after) / len(after)
                    if fa - fb >= 0.15 and fa >= 2 * fb:
                        out.append(f"After {what} at {clock(tc)}, the failure rate rose from "
                                   f"{fb:.0%} to {fa:.0%} over the next five minutes.")
                    elif fb - fa >= 0.15 and fb >= 2 * fa:
                        out.append(f"After {what} at {clock(tc)}, the failure rate fell from "
                                   f"{fb:.0%} to {fa:.0%} over the next five minutes.")

        # anomalies flagged by code
        recent = [a for a in anomalies if a["t"] >= start]
        if recent:
            words = {"new_event_type": "first appearance of {d}",
                     "unseen_transition": "a sequence not seen before ({d})",
                     "update_for_unknown_session": "a session update for an unknown session ({d})",
                     "close_for_unknown_session": "a close for an unknown session ({d})",
                     "double_close": "a session closed twice ({d})",
                     "failure_burst": "a burst of failures ({d})",
                     "unknown_result_code": "an unknown result code ({d})",
                     "unparseable_result": "a result that could not be read ({d})"}
            items = []
            for a in recent[-6:]:
                d = a["detail"]
                if a["kind"] == "new_event_type": d = event_text(d)
                if a["kind"] == "unseen_transition":
                    x, _, y = d.partition(" -> ")
                    d = f"{event_text(x)} followed by {event_text(y)}"
                items.append(f"{words.get(a['kind'], a['kind'] + ' ({d})').format(d=d)} at {clock(a['t'])}")
            out.append("Unusual events: " + "; ".join(items) + ".")
        else:
            out.append("Nothing unusual was flagged.")

        # monitored properties
        if hyps:
            good = [h for h in hyps.values() if h.get("status") == "holds"]
            weak = [h for h in hyps.values() if h.get("status") in ("likely", "refuted")]
            if good:
                out.append("Properties that continue to hold: " +
                           "; ".join(f"{prop_text(h['prop'])} ({h['sat']} of {h['sat']} cases)" for h in good) + ".")
            for h in weak:
                n = h["sat"] + h["viol"]
                out.append(f"The property that {prop_text(h['prop'])} was violated in "
                           f"{h['viol']} of {n} cases ({h['status']}).")

        self.history["opened"].append(len(opened))
        self.history["results"].append(len(rcodes))
        if rcodes: self.history["fail_rate"].append(rate)
        return "\n".join(out)


SYSTEM = J.SYSTEM.split("Propose temporal")[0] + """You receive a plain-language situation
report about the server rather than raw log data. Assess the situation:
is anything wrong, and is it operational or security-relevant? Then propose
temporal properties worth monitoring, using the vocabulary below.
""" + "Event names" + J.SYSTEM.split("Event names", 1)[1]


def learn_round(st, hyps, narrator, model, once_window=None, out_path=None):
    text = narrator.describe(st, hyps, window_s=once_window)
    print(f"\n{'=' * 70}\n{text}\n")
    if model is None:
        return
    import ollama
    seen = sorted(st.counts)
    try:
        resp = ollama.chat(model=model, format=J.SCHEMA, options={"temperature": 0.2},
                           messages=[{"role": "system", "content": SYSTEM},
                                     {"role": "user", "content": text + "\n\nEvent types seen so far: "
                                      + ", ".join(seen)}])
        out = json.loads(resp["message"]["content"])
    except Exception as ex:
        print("ollama error:", ex)
        return
    for p in out.get("properties", []):
        key = json.dumps({k: v for k, v in p.items() if k != "rationale"}, sort_keys=True)
        hyps.setdefault(key, {"prop": p, "refuted_rounds": 0})
    for key, h in list(hyps.items()):
        h["sat"], h["viol"] = J.check(h["prop"], st)
        single = h["prop"].get("kind") == "result_fraction" or (
            h["prop"].get("kind") == "absence" and h["prop"].get("scope") != "session")
        h["status"] = J.status(h["sat"], h["viol"], min_support=1 if single else 5)
        h["refuted_rounds"] = h["refuted_rounds"] + 1 if h["status"] == "refuted" else 0
        if h["refuted_rounds"] >= 3:
            del hyps[key]
    print("Assessment:", out.get("anomaly_assessment", ""))
    print("Observations:", out.get("observations", ""))
    for h in hyps.values():
        print(f"  [{h['status']}] {prop_text(h['prop'])}")
    if out_path:
        with open(out_path, "w") as f:
            json.dump({"report": text, "assessment": out.get("anomaly_assessment", ""),
                       "properties": [{**h["prop"], "status": h["status"], "satisfied": h["sat"],
                                       "violated": h["viol"]} for h in hyps.values()]}, f, indent=2)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--model", default="qwen2.5:7b")
    ap.add_argument("--no-llm", action="store_true", help="only print the situation reports")
    ap.add_argument("--window", type=int, default=300, help="seconds per report")
    ap.add_argument("--stale", type=int, default=300)
    ap.add_argument("--out", default="jane_situation.json")
    sub = ap.add_subparsers(dest="source", required=True)
    m = sub.add_parser("mqtt")
    m.add_argument("--broker", default="localhost")
    m.add_argument("--port", type=int, default=1883)
    m.add_argument("--topic", default="#")
    m.add_argument("--quiet", action="store_true", help="don't print a line per received message")
    f = sub.add_parser("file")
    f.add_argument("path")
    f.add_argument("--from-start", action="store_true")
    f.add_argument("--once", action="store_true", help="report on the whole file once, then exit")
    args = ap.parse_args()

    st, hyps, nar = J.State(stale_s=args.stale), {}, Narrator(args.window)
    model = None if args.no_llm else args.model
    if args.source == "file" and args.once:
        args.from_start = True
        J.run_file(st, args)
        span = (st.trace[-1][0] - st.trace[0][0] + 1) if st.trace else None
        learn_round(st, hyps, nar, model, once_window=span, out_path=args.out)
        return

    def loop():
        while True:
            time.sleep(args.window)
            learn_round(st, hyps, nar, model, out_path=args.out)
    threading.Thread(target=loop, daemon=True).start()
    if args.source == "mqtt":
        run_mqtt_verbose(st, args)
    else:
        J.run_file(st, args)


def run_mqtt_verbose(st, args):
    """Like jane_log_learner.run_mqtt, but prints one line per message received."""
    import paho.mqtt.client as mqtt
    received = 0

    def on_connect(c, _u, _f, rc, _p):
        print(f"connected to {args.broker}:{args.port} ({rc}), subscribing to {args.topic}", flush=True)
        c.subscribe(args.topic)

    def on_disconnect(_c, _u, _f, rc, _p):
        print(f"disconnected ({rc}), paho will try to reconnect", flush=True)

    def on_message(_c, _u, msg):
        nonlocal received
        received += 1
        text = msg.payload.decode(errors="replace")
        e = J.parse(text)
        now = time.strftime("%H:%M:%S")
        if not args.quiet:
            if e is None:
                print(f"{now}  #{received:<5} {msg.topic:<7} UNPARSED: {text.strip()[:100]}", flush=True)
            else:
                detail = e["msg"] or e["refid"][:12]
                if J.event_type(e) == "R/add/result" and str(detail).lstrip("-").isdigit():
                    detail = f"{detail} ({J.RESULT_CODES.get(int(detail), 'unknown code')})"
                print(f"{now}  #{received:<5} {msg.topic:<7} {J.event_type(e):<26} {str(detail)[:60]}", flush=True)
        if e:
            st.ingest(e)

    c = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    c.on_connect, c.on_disconnect, c.on_message = on_connect, on_disconnect, on_message
    print(f"connecting to {args.broker}:{args.port} ...", flush=True)
    c.connect(args.broker, args.port)
    c.loop_forever()


if __name__ == "__main__":
    main()
