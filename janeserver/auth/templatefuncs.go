package auth

import (
	"html/template"

	"github.com/labstack/echo/v4"
)

// TemplateFuncs returns the auth-related functions to register when the Web UI templates are parsed:
//
//	authEnabled   true if the Web UI requires a login
//	currentUser   the logged-in username (a placeholder here; see RequestTemplateFuncs)
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"authEnabled": WebEnabled,
		"currentUser": func() string { return "" },
	}
}

// RequestTemplateFuncs returns the functions whose values depend on the request being rendered.
// The Web UI renderer binds these to a clone of the page template for each request.
func RequestTemplateFuncs(c echo.Context) template.FuncMap {
	return template.FuncMap{
		"currentUser": func() string { return CurrentUser(c) },
	}
}
