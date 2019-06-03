package entropy

import (
	"fmt"
	"net/http"
)

type Lavarand struct {
}

func (*Lavarand) Read(data []byte) (n int, err error) {
	bytes := len(data)
	url := fmt.Sprintf("https://lavarand.ing.pdx-a.k8s.cfplat.com/entropy?bytes=%d", bytes)
	resp, errGetRandomness := http.Get(url)
	if errGetRandomness != nil {
		return 0, errGetRandomness
	}

	bytesRead, errReadBody := resp.Body.Read(data)
	if errReadBody != nil {
		return bytesRead, errReadBody
	}
	if bytesRead != len(data) {
		return bytesRead, fmt.Errorf("failed to read enough bytes from lavarand, got %d but expected %d",
			bytesRead, len(data))
	}

	return bytesRead, nil
}
