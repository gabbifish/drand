package lavarand

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func GetRandom(bytes uint32) ([]byte, error) {
	url := fmt.Sprintf("https://lavarand.ing.pdx-a.k8s.cfplat.com/entropy?bytes=%d", bytes)
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