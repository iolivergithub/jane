package restapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"a10/configuration"
	"a10/logging"

	"context"
)

const PREFIX = ""

//const PREFIX="/v3"

func StartRESTInterface(ctx context.Context) {
	router := echo.New()

	router.HideBanner = true

	//not necessary, but I will keep this here because this is now my example of how to use middlewares
	//in echo, plus the import declaration above
	router.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: 5}))
	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))
	//router.Use(middleware.Logger())

	//setup endpoints
	setupStatusEndpoints(router)
	setUpOperationEndpoints(router)
	setupAuxillaryOperationEndpoints(router)
	setupAttestationEndpoints(router)
	setUpLoggingEndpoints(router)

	//get configuration data
	port := ":" + configuration.ConfigData.Rest.Port
	crt := configuration.ConfigData.Rest.Crt
	key := configuration.ConfigData.Rest.Key
	usehttp := configuration.ConfigData.Rest.UseHTTP
	listenon := configuration.ConfigData.Web.ListenOn

	//start the server
	if usehttp == true {
		msg := fmt.Sprintf("REST HTTP mode starting, listening on %v at %v.", listenon, port)
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "RESTAPI", msg)
		go func() {
			if err := router.Start(port); err != nil && err != http.ErrServerClosed {
				router.Logger.Fatal("shutting down the server")
			}
		}()
	} else {
		msg := fmt.Sprintf("REST HTTP mode starting, listening on %v at %v.", listenon, port)
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "RESTAPI", msg)
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
	msg := fmt.Sprintf("RESI API graceful shutdown")
	logging.MakeLogEntry("SYS", "shutdown", configuration.ConfigData.System.Name, "RESTAPI", msg)

}

func setUpOperationEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/elements", getElements)
	router.GET(PREFIX+"/element/:itemid", getElement)
	router.GET(PREFIX+"/elements/name/:name", getElementsByName)
	router.GET(PREFIX+"/elements/tag/:tag", getElementsByTag)
	router.POST(PREFIX+"/element", postElement)
	router.PUT(PREFIX+"/element", putElement)
	router.DELETE(PREFIX+"/element/:itemid", deleteElement)

	router.GET(PREFIX+"/intents", getIntents)
	router.GET(PREFIX+"/intent/:itemid", getIntent)
	router.GET(PREFIX+"/intents/name/:name", getIntentsByName)
	router.POST(PREFIX+"/intent", postIntent)
	router.PUT(PREFIX+"/intent", putIntent)
	router.DELETE(PREFIX+"/intent/:itemid", deleteIntent)

	router.GET(PREFIX+"/expectedValues", getExpectedValues)
	router.GET(PREFIX+"/expectedValue/:itemid", getExpectedValue)
	router.GET(PREFIX+"/expectedValues/name/:name", getExpectedValuesByName)
	router.GET(PREFIX+"/expectedValues/element/:itemid", getExpectedValuesByElement)
	router.GET(PREFIX+"/expectedValues/intent/:itemid", getExpectedValuesByPolicy)
	router.GET(PREFIX+"/expectedValue/:eid/:pid", getExpectedValueByElementAndPolicy)

	router.POST(PREFIX+"/expectedValue", postExpectedValue)
	router.PUT(PREFIX+"/expectedValue", putExpectedValue)
	router.DELETE(PREFIX+"/expectedValue/:itemid", deleteExpectedValue)

	router.GET(PREFIX+"/claims", getClaims)
	router.GET(PREFIX+"/claim/:itemid", getClaim)
	router.GET(PREFIX+"/claims/element/:itemid", getClaimsByElementID)
	router.POST(PREFIX+"/claim", postClaim)

	router.GET(PREFIX+"/results", getResults)
	router.GET(PREFIX+"/result/:itemid", getResult)
	router.GET(PREFIX+"/results/element/:itemid", getResultsByElementID)
	router.POST(PREFIX+"/result", postResult)

	router.GET(PREFIX+"/sessions", getSessions)
	router.GET(PREFIX+"/session/:itemid", getSession)
	router.POST(PREFIX+"/session", postSession)
	router.DELETE(PREFIX+"/session/:itemid", deleteSession)

	router.PUT(PREFIX+"/session/:sid/claim/:cid", putSessionClaim)
	router.PUT(PREFIX+"/session/:sid/result/:rid", putSessionResult)

}

func setupAuxillaryOperationEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/protocols", getProtocols)
	router.GET(PREFIX+"/protocol/:name", getProtocol)

	router.GET(PREFIX+"/rules", getRules)
	router.GET(PREFIX+"/rule/:name", getRule)

	router.GET(PREFIX+"/opaqueobjects", getOpaqueObjects)
	router.GET(PREFIX+"/opaqueobject/:value", getOpaqueObjectByValue)
	router.POST(PREFIX+"/opaqueobject", postOpaqueObject)
	router.PUT(PREFIX+"/opaqueobject", putOpaqueObject)
	router.DELETE(PREFIX+"/opaqueobject/:value", deleteOpaqueObject)

}

func setupAttestationEndpoints(router *echo.Echo) {
	router.POST(PREFIX+"/attest", postAttest)
	router.POST(PREFIX+"/verify", postVerify)
}

func setupStatusEndpoints(router *echo.Echo) {
	router.GET(PREFIX+"/", homepage)
	router.GET(PREFIX+"/config", config)
	router.GET(PREFIX+"/health", health)
}

func setUpLoggingEndpoints(router *echo.Echo) {
	//other endpoint will be put here
	router.GET(PREFIX+"/log", getLogEntries)
	router.GET(PREFIX+"/log/since", getLogEntriesSince)
}

type homepageData struct {
	Name           string `json:"name" xml:"name"`
	WelcomeMessage string `json:"welcomeMessage" xml:"welcomeMessage"`
	Prefix         string `json:"prefix" xml:"prefix"`
}

func homepage(c echo.Context) error {
	h := homepageData{"Jane", "Croeso, Tervetuola, Welcome", PREFIX}

	return FormattedResponse(c, http.StatusOK, h)
}

func config(c echo.Context) error {
	return FormattedResponse(c, http.StatusOK, configuration.ConfigData)
}
