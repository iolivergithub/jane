package janeapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"tantor/provisioningfile"
	"tantor/structures"
)

func GetServerStatus() string {
	url := provisioningfile.ProvisioningData.AttestationServer + "/"

	resp, err := http.Get(url)

	fmt.Println("URL %v\n", url)

	if err != nil {
		panic("Request to Jane failed.")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic("Reading Jane welcome message failed")
	}

	bodyString := string(body)
	return bodyString
}

type postAddElementReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

func AddElement(e structures.Element) (string, error) {
	url := provisioningfile.ProvisioningData.AttestationServer + "/element"

	jstr, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	fmt.Println(string(jstr))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jstr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	var j postAddElementReturn
	err = json.Unmarshal([]byte(body), &j)
	if err != nil {
		fmt.Printf("failed to unmarhall body %v\n", body)
	}
	fmt.Println("*********************")
	fmt.Printf("Element ID is %v\n", j.Itemid)

	return j.Itemid, err

	return string(body), nil
}

type AttestStr struct {
	EID        string                 `json:"eid"`
	EPN        string                 `json:"epn"`
	PID        string                 `json:"pid"`
	SID        string                 `json:"sid"`
	Parameters map[string]interface{} `json:"parameters" bson:"parameters"`
}

func Attest(a AttestStr) (string, error) {
	url := provisioningfile.ProvisioningData.AttestationServer + "/attest"

	jstr, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	fmt.Println(string(jstr))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jstr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return string(body), nil
}

type postSessionMessage struct {
	Message string `json:"message"`
}

type postSessionReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

func OpenSession(m string) (string, error) {
	url := provisioningfile.ProvisioningData.AttestationServer + "/session"
	msg := postSessionMessage{m}

	jstr, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	fmt.Println(string(jstr))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jstr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	var j postSessionReturn
	err = json.Unmarshal([]byte(body), &j)
	if err != nil {
		fmt.Printf("failed to unmarhall body %v\n", body)
	}
	fmt.Println("*********************")
	fmt.Printf("Session ID is %v\n", j.Itemid)

	return j.Itemid, err
}

func CloseSession(s string) (string, error) {
	url := provisioningfile.ProvisioningData.AttestationServer + "/session/" + s

	req, err := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return string(body), nil
}
