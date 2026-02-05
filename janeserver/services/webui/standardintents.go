package webui

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	//"crypto/sha256"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

const STDINTENTSURL = "https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.json"
const STDINTENTSSHA256 = "https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.sha256"

func loadstandardintents(c echo.Context) error {
	var intentfilestruct []structures.Intent

	stdintents, errints := getfile(STDINTENTSURL)
	//stdsha256, err256  := getfile( STDINTENTSSHA256 )

	if errints != nil {
		fmt.Printf("Loading error: I=%v  \n", errints.Error())
		return c.Redirect(http.StatusSeeOther, "/intents")
	}

	// now parse the JSON string of the intents into a struct

	errjson := json.Unmarshal(stdintents, &intentfilestruct)
	if errjson != nil {
		fmt.Printf("error unmarshalling JSON %v\n", errjson.Error())
	}

	for _, e := range intentfilestruct {
		operations.AddStandardIntent(e)
	}

	return c.Redirect(http.StatusSeeOther, "/intents")

}

func getfile(url string) ([]byte, error) {

	// The get to the https URL is broken due to x509 certs being old...maybe a LInux issue, maybe a Windows issue
	// maybe a Github issue...anyway...maybe this solves it
	// Yes, I know this is a security hole.
	//
	// https://stackoverflow.com/questions/12122159/how-to-do-a-https-request-with-bad-certificate
	// https://stackoverflow.com/questions/12122159/how-to-do-a-https-request-with-bad-certificate

	// this was the original line:
	// resp, err := http.Get(url)
	// now replaced with...

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url)

	// and back to the original code here
	if err != nil {
		fmt.Printf("getting std intents failed: %w \n ", err.Error())
		return []byte{}, err
	}

	fmt.Println("Received a response")

	respbody, err2 := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err2 != nil {
		return []byte{}, err2
	} else {
		return respbody, nil
	}
}
