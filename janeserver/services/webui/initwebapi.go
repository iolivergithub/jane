package webui

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"a10/configuration"
	"a10/logging"

	"context"
)

// file embedding
//
//go:embed templates/*.html
var WPFS embed.FS

const PREFIX = ""
const T = "templates/"

type TemplateRegistry struct {
	templates map[string]*template.Template
}

func (t *TemplateRegistry) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	tmpl, ok := t.templates[name]
	if !ok {
		return fmt.Errorf("Error rendering %v with %v\n", name, data)
	}

	return tmpl.ExecuteTemplate(w, "base.html", data)
}

func StartWebUI(ctx context.Context) {
	templates := parseTemplates(templateFunctions())

	// Create the router
	router := echo.New()

	router.HideBanner = true
	router.Renderer = &TemplateRegistry{
		templates: templates,
	}

	// Middlewares
	router.Use(middleware.Logger())
	router.Use(middleware.Secure())

	//Ignore this as it causes issues with nginx and rewriting of URLs
	//Easier to let nginx deal with this anyway
	//router.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: 5}))

	//setup endpoints
	setupHomeEndpoints(router)
	setUpDisplayEndpoints(router)
	setUpAttestationEndpoints(router)
	setupEditEndpoints(router)

	//get configuration data
	port := ":" + configuration.ConfigData.Web.Port
	crt := configuration.ConfigData.Web.Crt
	key := configuration.ConfigData.Web.Key
	usehttp := configuration.ConfigData.Web.UseHTTP
	listenon := configuration.ConfigData.Web.ListenOn

	//start the server
	if usehttp == true {
		msg := fmt.Sprintf("WEB UI HTTP mode starting, listening on %v at %v.", listenon, port)
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "WEBUI", msg)
		go func() {
			if err := router.Start(port); err != nil && err != http.ErrServerClosed {
				router.Logger.Fatal("shutting down the server")
			}
		}()
	} else {
		msg := fmt.Sprintf("WEB UI HTTPS mode starting, listening on %v at %v.", listenon, port)
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "WEBUI", msg)
		go func() {
			if err := router.StartTLS(port, crt, key); err != nil && err != http.ErrServerClosed {
				router.Logger.Fatal("shutting down the server")
			}
		}()
	}

	// Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := router.Shutdown(ctx); err != nil {
		router.Logger.Fatal(err)
	}

	msg := fmt.Sprintf("WEB UI graceful shutdown")
	logging.MakeLogEntry("SYS", "shutdown", configuration.ConfigData.System.Name, "WEBUI", msg)
}

func setupEditEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/new/element", newElement)
	router.POST(PREFIX+"/new/element", processNewElement)

	router.GET(PREFIX+"/new/intent", newIntent)
	router.POST(PREFIX+"/new/intent", processNewIntent)

	router.GET(PREFIX+"/new/expectedvalue", newExpectedValue)
	router.POST(PREFIX+"/new/expectedvalue", processNewExpectedValue)

	router.GET(PREFIX+"/loadstandardintents", loadstandardintents)

	router.GET(PREFIX+"/new/opaqueobject", newOpaqueObject)
	router.POST(PREFIX+"/new/opaqueobject", processOpaqueObject)
}

func setupHomeEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/", homepage)
	router.GET(PREFIX+"/help", helppage)
	router.GET(PREFIX+"/about", aboutpage)
}

func setUpDisplayEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/elements", showElements)
	router.GET(PREFIX+"/element/:itemid", showElement)
	router.GET(PREFIX+"/intents", showIntents)
	router.GET(PREFIX+"/intent/:itemid", showIntent)
	router.GET(PREFIX+"/expectedvalues", showExpectedValues)
	router.GET(PREFIX+"/expectedvalue/:itemid", showExpectedValue)

	router.GET(PREFIX+"/claims", showClaims)
	router.GET(PREFIX+"/claim/:itemid", showClaim)

	router.GET(PREFIX+"/results", showResults)
	router.GET(PREFIX+"/result/:itemid", showResult)

	// sessions
	router.GET(PREFIX+"/sessions", showSessions)
	router.GET(PREFIX+"/session/:itemid", showSession)

	// protocols

	router.GET(PREFIX+"/protocols", showProtocols)

	// opaqueobjects
	router.GET(PREFIX+"/opaqueobjects", showOpaqueObjects)
	router.GET(PREFIX+"/opaqueobject/:name", showOpaqueObject)

	// rules
	router.GET(PREFIX+"/rules", showRules)

	//log
	router.GET(PREFIX+"/log", showLog)
	router.GET(PREFIX+"/log/since", showLogSince)

	//live
	router.GET(PREFIX+"/live", showLive)

}

func setUpAttestationEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/attest", showAttest)
	router.POST(PREFIX+"/attest", processAttest)

}
