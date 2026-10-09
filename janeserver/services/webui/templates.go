package webui

import (
	"html/template"
)

// templateFunctions are the functions available to every template.
func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"defaultMessage": DefaultMessage, "epochToUTCdetailed": EpochToUTCdetailed, "epochToUTC": EpochToUTCsimple, "base64decode": Base64decode,
		"encodeAsHexString": EncodeAsHexString, "tcgAlg": TCGAlg, "opaqueObjectInt64": GetOpaqueObjectByValueInt64,
		"opaqueObject": GetOpaqueObjectByValue,
		"resultClass":  ResultClass, "resultLabel": ResultLabel, "duration": Duration, "shortTime": ShortTime,
	}
}

// parseTemplates parses every page template with its partials and base.html.
//
// It is done this way so we can have different templates for each operation...a bit ugly, but html/template is not jinja
// dev.to/ykyuen/setup-nested-html-template-in-go-echo-web-framework-d9b
func parseTemplates(functions template.FuncMap) map[string]*template.Template {
	templates := make(map[string]*template.Template)

	page := func(name string, partials ...string) {
		files := []string{T + name, T + "base.html"}
		for _, p := range partials {
			files = append(files, T+p)
		}
		templates[name] = template.Must(template.New(name).Funcs(functions).ParseFS(WPFS, files...))
	}

	page("home.html")
	page("help.html")
	page("about.html")
	page("live.html")

	page("elements.html", "elementsummarylist.html")
	page("element.html", "uefi.html", "txt.html", "ima.html", "tpm2.html", "tpm2key.html",
		"hostinformation.html", "recordhistory.html", "resultvalue.html")

	page("intents.html", "intentsummarylist.html")
	page("intent.html", "genericList.html")

	page("evs.html", "evsummarylist.html", "recordhistory.html")
	page("ev.html", "recordhistory.html", "genericList.html", "resultvalue.html")

	page("claims.html", "pager.html")
	page("claim.html", "claim_ERROR.html", "claim_ima.html", "claim_quote.html", "claim_tpm2pcrs.html",
		"claim_efivars.html", "genericList.html", "resultvalue.html")

	page("results.html", "resultvalue.html", "pager.html")
	page("result.html", "resultvalue.html", "genericList.html")

	page("sessions.html", "pager.html")
	page("session.html", "resultvalue.html")

	page("attest.html")
	page("protocols.html")
	page("rules.html")
	page("log.html")

	page("opaqueobjects.html")
	page("opaqueobject.html")

	page("editelement.html")
	page("editintent.html")
	page("editexpectedvalue.html")
	page("editopaqueobject.html")

	return templates
}
