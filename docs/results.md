# Results

A result is the outcome of running one rule on one claim. Jane stores it in the `results` collection and adds it to the session it was run in.

| Field | Meaning |
|---|---|
| `itemid` | the result's identifier |
| `claimid`, `claimfooter` | the claim checked, and its footer (so the result refers to exactly that claim) |
| `session` | the session the rule was run in |
| `elementid` | the claim's element |
| `rulename` | the rule run |
| `parameters` | the rule parameters given |
| `expectedvalue` | the expected value used, if the rule needs one (a full copy, as it was) |
| `result` | the outcome: 0 pass, 9001 fail, or an error value (see [Rules](rules.md#result-values)) |
| `message` | the rule's explanation, e.g. what differed |
| `verifiedat` | when, nanoseconds since 1970 (UTC) |
| `footer` | hash and signature over the above (see [Signing](signing.md)) |

A rule may be run on a claim any number of times, in the same or a later session; each run makes a new result. The Web UI shows results on their session's page, on the claim's page, and on the expected value's page.

The Go definition is `janeserver/structures/results.go`.
