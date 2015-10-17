package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type StatusCheckAction struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Output string `json:"output"`
}

type StatusResponse struct {
	Data StatusData `json:"data"`
}

type StatusData struct {
	Status string                  `json:"status"`
	Detail map[string]StatusDetail `json:"detail"`
}

type StatusDetail struct {
	Status string `json:"status"`
}

func init() {
	Register("status-check", &StatusCheckAction{})
}

func (a *StatusCheckAction) Run(s *State) error {
	const waitMax = time.Minute
	const waitInterval = 500 * time.Millisecond

	u, err := url.Parse(interpolate(s, a.URL))
	if err != nil {
		return err
	}
	lookupDiscoverdURLHost(u, waitMax)

	start := time.Now()
	for {
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return err
		}
		req.Header = make(http.Header)
		req.Header.Set("Accept", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			goto fail
		}
		if res.StatusCode == 200 {
			s.StepData[a.ID] = &LogMessage{Msg: "all services healthy"}
		} else if time.Now().Sub(start) < waitMax {
			// if services are unhealthy wait out the wait period
			// before reporting them as unhealthy to the user
			time.Sleep(waitInterval)
			continue
		} else {
			var status StatusResponse
			err = json.NewDecoder(res.Body).Decode(&status)
			if err != nil {
				goto fail
			}
			res.Body.Close()
			msg := "unhealthy services detected!\n\nThe following services are reporting unhealthy, this likely indicates a problem with your deployment:\n"
			for svc, s := range status.Data.Detail {
				if s.Status != "healthy" {
					msg += "\t" + svc + "\n"
				}
			}
			msg += "\n"
			s.StepData[a.ID] = &LogMessage{Msg: msg}
		}
		return nil
	fail:
		if time.Now().Sub(start) >= waitMax {
			return fmt.Errorf("bootstrap: timed out waiting for %s, last response %s", a.URL, err)
		}
		time.Sleep(waitInterval)
	}
}
