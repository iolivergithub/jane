package uefi

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "reflect"

	utilities "ta10/common"

	"github.com/labstack/echo/v4"

	"github.com/0x5a17ed/itkit/itlib"
	_ "github.com/0x5a17ed/uefi/efi/efiguid"
	"github.com/0x5a17ed/uefi/efi/efivario"
	"github.com/0x5a17ed/uefi/efi/efivars"
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

type efivardata struct {
	Name       string `json:"name"`
	Guid       string `json:"guid"`
	Attributes int32  `json:"attributes"`
	SomeValue  int    `json:"somevalue"` // no idea what this is supposed to represent
	Value      string `json:"value"`
	ErrorValue string `json:"errorvalue"`
}

type returnEfivars struct {
	Efivars []efivardata `json:"efivars"`
	Count   int          `json:"count"`
}

func Efivars(c echo.Context) error {
	fmt.Println("efivars called")

	var efivars = []efivardata{}

	co := efivario.NewDefaultContext()
	vni, _ := co.VariableNames()

	itlib.Apply(vni.Iter(), func(v efivario.VariableNameItem) {
		hint, err := co.GetSizeHint(v.Name, v.GUID)
		if err != nil || hint < 0 {
			hint = 8
		}

		out := make([]byte, hint)
		a, i, err := co.Get(v.Name, v.GUID, out)
		//fmt.Printf("N=%v, A=%v, I=%v, E=%v, S=%v, O=%v, X=%v\n\n",v.Name,a,i,err,string(out[:]),out,err)
		//fmt.Printf("Types %v, %v, %v, %v, %v\n",reflect.TypeOf(v.Name),reflect.TypeOf(v.GUID.String()), reflect.TypeOf(int32(a)), reflect.TypeOf(i), reflect.TypeOf(string(out[:])))
		errval := ""
		if err != nil {
			errval = err.Error()
		}

		efd := efivardata{v.Name, v.GUID.String(), int32(a), i, string(out[:]), errval}

		fmt.Printf("X=%v\n\n", efd)

		efivars = append(efivars, efd)
	})

	rtn := returnEfivars{efivars, len(efivars)}

	return c.JSON(http.StatusOK, rtn)
}

type returnBootInformation struct {
	Message string `json:"msg"`
}

func BootConfig(c echo.Context) error {
	fmt.Println("boot order called")

	co := efivario.NewDefaultContext()

	v1, v2, v3 := efivars.BootCurrent.Get(co)
	fmt.Printf("EFIbootcurrent %v\n%v\n%v\n", v1, v2, v3)

	ov1, ov2, ov3 := efivars.BootOrder.Get(co)
	fmt.Printf("EFIbootorder %v\n%v\n%v\n", ov1, ov2, ov3)

	bv1, bv2, bv3 := efivars.BootNext.Get(co)
	fmt.Printf("EFIbootnext %v\n%v\n%v\n", bv1, bv2, bv3)

	rtn := returnBootInformation{"boot order not implemented yet"}

	fmt.Printf("rtn=%v\n", rtn)

	return c.JSON(http.StatusOK, rtn)
}
