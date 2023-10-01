package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/alecthomas/kong"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type CLI struct {
	MQTTBroker      string        `help:"MQTT broker address" default:"tcp://localhost:1883"`
	MQTTUsername    string        `help:"MQTT username" default:""`
	MQTTPassword    string        `help:"MQTT password" default:""`
	TibberUsername  string        `help:"Tibber username" default:""`
	TibberPassword  string        `help:"Tibber password" default:""`
	RefreshInterval time.Duration `help:"Refresh interval" default:"2m"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cli.MQTTBroker)
	opts.SetClientID("calmh.dev/tibberevmqtt")
	if cli.MQTTUsername != "" && cli.MQTTPassword != "" {
		opts.SetUsername(cli.MQTTUsername)
		opts.SetPassword(cli.MQTTPassword)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Connect to MQTT broker:", token.Error())
	}

	if err := announceMQTT(client); err != nil {
		log.Fatal("Announce MQTT:", err)
	}

	svc := tibberSvc{
		username: cli.TibberUsername,
		password: cli.TibberPassword,
	}
	t := time.NewTimer(time.Second)
	for range t.C {
		t.Reset(cli.RefreshInterval)
		soc, err := svc.getEVSoC(context.Background())
		if err != nil {
			log.Println("Get EV SoC:", err)
			continue
		}
		payload, err := json.Marshal(map[string]int{"soc": soc})
		if err != nil {
			log.Println("Marshal SoC:", err)
			continue
		}
		token := client.Publish("homeassistant/sensor/tibberEV0/state", 0, false, string(payload))
		token.Wait()
		if token.Error() != nil {
			log.Println("Publish SoC:", token.Error())
		}
	}
}

func announceMQTT(client mqtt.Client) error {
	cfgTopic := "homeassistant/sensor/tibberEV0/config"
	cfgPayload := map[string]any{
		"device_class":        "battery",
		"state_topic":         "homeassistant/sensor/tibberEV0/state",
		"unit_of_measurement": "%",
		"value_template":      "{{ value_json.soc }}",
		"unique_id":           "batteryev0",
		"device": map[string]any{
			"identifiers": []string{"ev0"},
			"name":        "EV",
		},
	}
	cfg, err := json.Marshal(cfgPayload)
	if err != nil {
		return err
	}
	token := client.Publish(cfgTopic, 0, false, cfg)
	token.Wait()
	return token.Error()
}
