package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type tibberSvc struct {
	username string
	password string
	token    string
}

func (t *tibberSvc) authenticate(ctx context.Context) error {
	js, err := json.Marshal(map[string]string{"email": t.username, "password": t.password})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://app.tibber.com/login.credentials", bytes.NewReader(js))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: %s", resp.Status)
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}
	t.token = authResp.Token
	return nil
}

type EVSoC struct {
	ID         string
	Name       string
	Percent    int
	IsCharging bool
	LastSeen   time.Time
}

func (t *tibberSvc) getEVSoC(ctx context.Context) ([]EVSoC, error) {
	if t.token == "" {
		if err := t.authenticate(ctx); err != nil {
			return nil, err
		}
	}

	// curl https://app.tibber.com/v4/gql -H "Authorization: Bearer 7te7m8FymxgYAp4qAiItSUWLwoqqBoWXKXOcWeaGgJI" -H "Content-Type: application/json" -d '{ "query": "{ me { homes { electricVehicles { lastSeen battery { percent } } } }}"}'
	js, err := json.Marshal(map[string]interface{}{
		"query": "{ me { homes { electricVehicles { id name lastSeen battery { percent isCharging } } } }}",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://app.tibber.com/v4/gql", bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.token)

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.token = ""
		return nil, fmt.Errorf("query failed: %s", resp.Status)
	}

	var respType struct {
		Data struct {
			Me struct {
				Homes []struct {
					ElectricVehicles []struct {
						ID       string
						Name     string
						LastSeen time.Time
						Battery  struct {
							Percent    float64
							IsCharging bool
						}
					}
				}
			}
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&respType); err != nil {
		return nil, err
	}
	if len(respType.Data.Me.Homes) == 0 || len(respType.Data.Me.Homes[0].ElectricVehicles) == 0 {
		return nil, fmt.Errorf("no EV found")
	}

	var evs []EVSoC
	for _, ev := range respType.Data.Me.Homes[0].ElectricVehicles {
		evs = append(evs, EVSoC{
			ID:         ev.ID,
			Name:       ev.Name,
			Percent:    int(ev.Battery.Percent),
			IsCharging: ev.Battery.IsCharging,
			LastSeen:   ev.LastSeen,
		})
	}

	return evs, nil
}
