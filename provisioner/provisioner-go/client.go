package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the Jane attestation server's REST API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(base string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(base, "/"),
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

type itemIDResponse struct {
	ItemID string `json:"itemid"`
}

// ServerError is a request the Jane server answered, but not with what was
// asked for (an error status, or no itemid). Transport failures, where the
// server could not be reached at all, are returned as other errors.
type ServerError struct {
	Method, Path string
	Status       int
	Body         string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("%s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// do sends body (if non-nil) as JSON and returns the status code and raw
// response body. Non-2xx statuses are not treated as errors here.
func (c *Client) do(method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	return resp.StatusCode, rb, err
}

// doItem sends a request whose response carries an itemid.
func (c *Client) doItem(method, path string, body any) (int, string, error) {
	status, rb, err := c.do(method, path, body)
	if err != nil {
		return status, "", err
	}
	var r itemIDResponse
	if err := json.Unmarshal(rb, &r); err != nil || r.ItemID == "" {
		return status, "", &ServerError{method, path, status, "no itemid in response: " + strings.TrimSpace(string(rb))}
	}
	return status, r.ItemID, nil
}

func (c *Client) OpenSession(msg string) (string, error) {
	_, sid, err := c.doItem(http.MethodPost, "/session", map[string]string{"message": msg})
	if err != nil {
		return "", fmt.Errorf("opening session: %w", err)
	}
	fmt.Println("session is", sid)
	return sid, nil
}

func (c *Client) CloseSession(sid string) error {
	status, _, err := c.do(http.MethodDelete, "/session/"+url.PathEscape(sid), nil)
	if err != nil {
		return fmt.Errorf("closing session: %w", err)
	}
	fmt.Println("session close", status)
	return nil
}

// CreateElement POSTs a new element and returns its itemid.
func (c *Client) CreateElement(e *Element) (string, error) {
	status, id, err := c.doItem(http.MethodPost, "/element", e)
	fmt.Println("Result is", status, http.StatusText(status), id)
	return id, err
}

// UpdateElement PUTs an element that already carries its itemid.
func (c *Client) UpdateElement(e *Element) (string, error) {
	status, id, err := c.doItem(http.MethodPut, "/element", e)
	fmt.Println("Result is", status, http.StatusText(status), id)
	return id, err
}

type attestRequest struct {
	EID        string         `json:"eid"`
	PID        string         `json:"pid"`
	EPN        string         `json:"epn"`
	SID        string         `json:"sid"`
	Parameters map[string]any `json:"parameters"`
}

// Attest asks the server to attest element eid against intent pid on endpoint
// epn, returning the claim ID.
func (c *Client) Attest(eid, pid, epn, sid string) (string, error) {
	status, cid, err := c.doItem(http.MethodPost, "/attest", attestRequest{
		EID: eid, PID: pid, EPN: epn, SID: sid, Parameters: map[string]any{},
	})
	fmt.Println(" ... attest ... ", status)
	return cid, err
}

// Claim is the part of a janeserver claim the provisioner reads.
type Claim struct {
	ItemID string          `json:"itemid"`
	Body   json.RawMessage `json:"body"`
}

func (c *Client) GetClaim(cid string) (*Claim, error) {
	status, rb, err := c.do(http.MethodGet, "/claim/"+url.PathEscape(cid), nil)
	fmt.Println(" ... claim ... ", status, cid)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, &ServerError{http.MethodGet, "/claim/" + cid, status, strings.TrimSpace(string(rb))}
	}
	var cl Claim
	if err := json.Unmarshal(rb, &cl); err != nil {
		return nil, &ServerError{http.MethodGet, "/claim/" + cid, status, "bad claim: " + err.Error()}
	}
	return &cl, nil
}

type verifyRequest struct {
	CID  string `json:"cid"`
	SID  string `json:"sid"`
	Rule string `json:"rule"`
}

// Result codes from janeserver's structures.ResultValue.
const (
	ResultSuccess                 = 0
	ResultFail                    = 9001
	ResultVerifyCallFailure       = 9010
	ResultVerifyClaimErrorAttempt = 9098
	ResultNoResult                = 9099
	ResultMissingExpectedValue    = 9997
	ResultRuleCallFailure         = 9998
	ResultUnsetResultValue        = 9999
)

var resultNames = map[int]string{
	ResultSuccess:                 "Success",
	ResultFail:                    "Fail",
	ResultVerifyCallFailure:       "VerifyCallFailure",
	ResultVerifyClaimErrorAttempt: "VerifyClaimErrorAttempt",
	ResultNoResult:                "NoResult",
	ResultMissingExpectedValue:    "MissingExpectedValue",
	ResultRuleCallFailure:         "RuleCallFailure",
	ResultUnsetResultValue:        "UnsetResultValue",
}

// ResultName names a janeserver result code, e.g. "9001 Fail".
func ResultName(code int) string {
	if n, ok := resultNames[code]; ok {
		return fmt.Sprintf("%d %s", code, n)
	}
	return fmt.Sprintf("%d unknown result", code)
}

// VerifyResult is janeserver's answer to POST /verify.
type VerifyResult struct {
	ResultID string `json:"itemid"` // the stored result, if one was stored
	Result   *int   `json:"result"` // nil if the server gave no result
	Error    string `json:"error"`
	Status   int    `json:"-"`
	Body     string `json:"-"` // raw response, kept when there is no result
}

// Passed reports whether the rule succeeded.
func (v *VerifyResult) Passed() bool { return v.Result != nil && *v.Result == ResultSuccess }

// Describe summarises the outcome, e.g. "9001 Fail" or "no result (HTTP 500: ...)".
func (v *VerifyResult) Describe() string {
	if v.Result == nil {
		return fmt.Sprintf("no result (HTTP %d: %s)", v.Status, v.Body)
	}
	d := ResultName(*v.Result)
	if v.Error != "" {
		d += ": " + v.Error
	}
	return d
}

// Verify runs rule against claim cid. A rule that fails, or that the server
// could not run, is reported in the VerifyResult; an error is returned only if
// the server could not be reached.
func (c *Client) Verify(cid, sid, rule string) (*VerifyResult, error) {
	status, rb, err := c.do(http.MethodPost, "/verify", verifyRequest{CID: cid, SID: sid, Rule: rule})
	if err != nil {
		return nil, err
	}
	v := &VerifyResult{Status: status}
	if jerr := json.Unmarshal(rb, v); jerr != nil || v.Result == nil {
		v.Result = nil
		v.Body = strings.TrimSpace(string(rb))
	}
	return v, nil
}

// ExpectedValue matches janeserver's structures.ExpectedValue.
type ExpectedValue struct {
	ItemID       string         `json:"itemid,omitempty"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	ElementID    string         `json:"elementid"`
	EndpointName string         `json:"endpointname"`
	IntentID     string         `json:"intentid"`
	EVS          map[string]any `json:"evs"`
}

// FindExpectedValue returns the itemid of the expected value for
// (eid, pid, epn), or found=false if the server has none (any non-200).
func (c *Client) FindExpectedValue(eid, pid, epn string) (id string, found bool, err error) {
	path := "/expectedValue/" + url.PathEscape(eid) + "/" + url.PathEscape(pid) + "/" + url.PathEscape(epn)
	fmt.Println(" ... evs, checking:", c.BaseURL+path)
	status, rb, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return "", false, err
	}
	fmt.Println(" ... status ", status)
	if status != http.StatusOK {
		return "", false, nil
	}
	var r itemIDResponse
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", false, fmt.Errorf("expected value lookup: %w", err)
	}
	fmt.Println(" ... evs", r.ItemID)
	return r.ItemID, true, nil
}

func (c *Client) CreateExpectedValue(ev *ExpectedValue) error {
	status, rb, err := c.do(http.MethodPost, "/expectedValue", ev)
	if err != nil {
		return err
	}
	fmt.Println(" ... evs ... ", status, strings.TrimSpace(string(rb)))
	return nil
}

func (c *Client) UpdateExpectedValue(ev *ExpectedValue) error {
	status, _, err := c.do(http.MethodPut, "/expectedValue", ev)
	if err != nil {
		return err
	}
	fmt.Println(" ... evs ... ", status)
	return nil
}
