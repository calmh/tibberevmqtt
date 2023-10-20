package main

import (
	"context"
	"log"
	"time"

	"calmh.dev/hassmqtt"
	"github.com/alecthomas/kong"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type CLI struct {
	MQTTBroker      string        `help:"MQTT broker address" default:"tcp://localhost:1883" env:"MQTT_BROKER"`
	MQTTUsername    string        `help:"MQTT username" default:"" env:"MQTT_USERNAME"`
	MQTTPassword    string        `help:"MQTT password" default:"" env:"MQTT_PASSWORD"`
	TibberUsername  string        `help:"Tibber username" default:"" env:"TIBBER_USERNAME"`
	TibberPassword  string        `help:"Tibber password" default:"" env:"TIBBER_PASSWORD"`
	RefreshInterval time.Duration `help:"Refresh interval" default:"2m" env:"REFRESH_INTERVAL"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	clientID := hassmqtt.ClientID("tibberevmqtt")
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cli.MQTTBroker)
	opts.SetClientID(clientID)
	if cli.MQTTUsername != "" && cli.MQTTPassword != "" {
		opts.SetUsername(cli.MQTTUsername)
		opts.SetPassword(cli.MQTTPassword)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Connect to MQTT broker:", token.Error())
	}

	svc := tibberSvc{
		username: cli.TibberUsername,
		password: cli.TibberPassword,
	}
	t := time.NewTimer(time.Second)

	metrics := make(map[string]*hassmqtt.Metric)

	for range t.C {
		t.Reset(cli.RefreshInterval)
		evs, err := svc.getEVSoC(context.Background())
		if err != nil {
			log.Println("Get EV SoC:", err)
			continue
		}
		for _, ev := range evs {
			m, ok := metrics[ev.ID]
			if !ok {
				m = &hassmqtt.Metric{
					Device: &hassmqtt.Device{
						Namespace: "tibberevmqtt",
						ClientID:  clientID,
						ID:        ev.ID,
						Name:      ev.Name,
					},
					ID:          "soc",
					DeviceType:  "sensor",
					DeviceClass: "battery",
					Unit:        "%",
				}
			}
			if err := m.Publish(client, ev.Percent); err != nil {
				log.Println("Publish:", err)
				continue
			}
		}
	}
}
