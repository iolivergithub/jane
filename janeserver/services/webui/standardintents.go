package webui


import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"

    //"crypto/sha256"

    "github.com/labstack/echo/v4"

    "a10/operations"
    "a10/structures"
)

const STDINTENTSURL="https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.json"
const STDINTENTSSHA256="https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.sha256"


func loadstandardintents(c echo.Context) error {
    var intentfilestruct []structures.Intent

    stdintents,errints := getfile( STDINTENTSURL )
    //stdsha256, err256  := getfile( STDINTENTSSHA256 )

    if errints!=nil {
        fmt.Printf("Loading error: I=%v  \n",errints.Error())   
        return  c.Redirect(http.StatusSeeOther, "/intents")
    }

    // now parse the JSON string of the intents into a struct

    errjson := json.Unmarshal(stdintents,&intentfilestruct)
    if errjson!=nil{
        fmt.Printf("error unmarshalling JSON %v\n",errjson.Error())
    }

    for _,e := range intentfilestruct {
        operations.AddStandardIntent(e)
    }

	return c.Redirect(http.StatusSeeOther, "/intents")

}




func getfile(url string) ( []byte, error ){

    resp,err := http.Get( url )

    if err!=nil {
        fmt.Println("getting std intents failed ", err.Error()) 
        return []byte{},err
    }

    respbody,err2 := ioutil.ReadAll(resp.Body)
    defer resp.Body.Close()

    if err2 != nil {
        return []byte{},err2
    } else {
        return respbody,nil
    }
}