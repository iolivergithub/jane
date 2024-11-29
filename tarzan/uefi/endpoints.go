package uefi

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	utilities "ta10/common"

	"github.com/labstack/echo/v4"


	"github.com/0x5a17ed/uefi/efi/efivario"
	"github.com/0x5a17ed/uefi/efi/efivars"	
	"github.com/0x5a17ed/uefi/efi/efiguid"	

	_ "github.com/0x5a17ed/itkit/iters/sliceit"
	"github.com/0x5a17ed/itkit/itlib"	
)

const UEFIEVENTLOGLOCATION string = "/sys/kernel/security/tpm0/binary_bios_measurements"

type returnEventLog struct {
	EventLog        string `json:"eventlog"`
	Encoding        string `json:"encoded"`
	UnEncodedLength int    `json:"unencodedlength"`
	EncodedLength   int    `json:"encodedlength"`
}

func GetEventLogLocation(loc string) string {
	fmt.Printf("UEFI Log requested from %v, unsafe mode is %v, giving: ", loc, utilities.IsUnsafe())

	if utilities.IsUnsafe() == true {
		fmt.Printf("%v\n", loc)
		return loc
	} else {
		fmt.Printf("%v\n", UEFIEVENTLOGLOCATION)
		return UEFIEVENTLOGLOCATION
	}
}

func Eventlog(c echo.Context) error {
	fmt.Println("eventlog called")

	var postbody map[string]interface{}
	var rtnbody = make(map[string]interface{})

	if err := c.Bind(&postbody); err != nil {
		rtnbody["postbody"] = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, rtnbody)
	}

	u := GetEventLogLocation(fmt.Sprintf("%v", postbody["uefi/eventlog"]))

	fcontent, err := ioutil.ReadFile(u)
	if err != nil {
		rtnbody["file err"] = err.Error()
		return c.JSON(http.StatusInternalServerError, rtnbody)
	}
	scontent := base64.StdEncoding.EncodeToString(fcontent)

	rtn := returnEventLog{scontent, "base64", len(fcontent), len(scontent)}
	return c.JSON(http.StatusOK, rtn)
}

type returnEfivars struct {
	Name        string    `json:"name"`
	Guid        efiguid.GUID    `json:"guid"`
	Attributes  efivario.Attributes     `json:"attributes"`
	SomeValue   int       `json:"somevalue"`
	StringValue string    `json:"stringvalue"`
	RawValue    []byte    `json:"rawvalue"`
	ErrorValue   string    `json:"errorvalue"`
}

func Efivars(c echo.Context) error {
	fmt.Println("efivars called")

	var rtnvars = []returnEfivars{}

	co := efivario.NewDefaultContext()
	vni,_ := co.VariableNames()

	itlib.Apply(vni.Iter(), func(v efivario.VariableNameItem){
			hint,err := co.GetSizeHint(v.Name,v.GUID)
			if err != nil || hint < 0 {
				hint = 8
			}

			out := make([]byte,hint)
			a,i,err := co.Get(v.Name,v.GUID,out)
			fmt.Printf("N=%v, A=%v, I=%v, E=%v, S=%v, O=%v, X=%v\n\n",v.Name,a,i,err,string(out[:]),out,err)

			errval := ""
			if err!=nil {
				errval = err.Error()
			}

			rtnvarstr :=returnEfivars{v.Name,v.GUID,a,i,string(out[:]),out,errval}

			rtnvars=append(rtnvars,rtnvarstr)
		})

	return c.JSON(http.StatusOK, rtnvars)
}

// type returnBootInformation struct {
// 	//TBD
// }

// func BootConfig(c echo.Context) error {
// 	fmt.Println("boot order called")

// 	var rtnvars = []returnEfivars{}

// 	v1,v2,v3 := efivars.BootCurrent.Get(co)
// 	fmt.Printf("EFI %v\n%v\n%v\n",v1,v2,v3)

// 	ov1,ov2,ov3 := efivars.BootOrder.Get(co)
// 	fmt.Printf("EFI %v\n%v\n%v\n",ov1,ov2,ov3)

// 	bv1,bv2,bv3 := efivars.BootNext.Get(co)
// 	fmt.Printf("EFI %v\n%v\n%v\n",bv1,bv2,bv3)


// 	return c.JSON(http.StatusOK, rtnvars)
// }