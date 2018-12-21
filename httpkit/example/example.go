package main

import (
	"fmt"
	"net/url"
)

func main() {
	str := "/flightx/hotelprice/hotelstat?a=1"
	u, err := url.ParseRequestURI(str)
	fmt.Println(u, err)
	fmt.Println(u.RawQuery, u.RawQuery == "")
	// //rq := u.RawQuery
	// vals, e := url.ParseQuery("a=")
	// fmt.Println(vals, e)
	//fmt.Println(AdvanceClient())
}
