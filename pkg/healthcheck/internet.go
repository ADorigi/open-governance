package healthcheck

import (
	"fmt"
	"net/http"
)

func InternetURLCheck(url string) error {

	client := http.Client{}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("InternetURLCheck: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	} else {
		return fmt.Errorf("InternetURLCheck: Could not connect to %s", url)
	}

}
