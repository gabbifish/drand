package lavarand

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func GetRandom(bytes uint32) ([]byte, error) {
	url := fmt.Sprintf("https://lavarand2.cfdata.org/entropy?bytes=%d", bytes)
	resp, errGetRandomness := http.Get(url)
	if errGetRandomness != nil {
		return nil, errGetRandomness
	}

	body, errReadBody := ioutil.ReadAll(resp.Body)
	if errReadBody != nil {
		return nil, errReadBody
	}

	return body, nil
}