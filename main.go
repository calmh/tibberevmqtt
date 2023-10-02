package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

	svc := tibberSvc{
		username: cli.TibberUsername,
		password: cli.TibberPassword,
	}
	t := time.NewTimer(time.Second)
	for range t.C {
		t.Reset(cli.RefreshInterval)
		evs, err := svc.getEVSoC(context.Background())
		if err != nil {
			log.Println("Get EV SoC:", err)
			continue
		}
		for _, ev := range evs {
			if err := announceMQTT(client, ev); err != nil {
				log.Println("Announce MQTT:", err)
			}
		}
	}
}

func announceMQTT(client mqtt.Client, ev EVSoC) error {
	id := strings.ReplaceAll(ev.ID, "-", "")
	stateTopic := fmt.Sprintf("tibberevmqtt/%s/state", id)
	state := map[string]any{
		"soc":      ev.Percent,
		"charging": ev.IsCharging,
	}
	if err := sendMQTT(client, stateTopic, state, true); err != nil {
		return err
	}

	socTopic := fmt.Sprintf("homeassistant/sensor/tibberEV%ssoc/config", id)
	socPayload := map[string]any{
		"device_class":        "battery",
		"state_topic":         stateTopic,
		"unit_of_measurement": "%",
		"value_template":      "{{ value_json.soc }}",
		"unique_id":           fmt.Sprintf("%ssoc", id),
		"device": map[string]any{
			"identifiers": []string{id},
			"name":        ev.Name,
		},
	}
	if err := sendMQTT(client, socTopic, socPayload, true); err != nil {
		return err
	}

	chargingTopic := fmt.Sprintf("homeassistant/binary_sensor/tibberEV%scharging/config", id)
	chargingPayload := map[string]any{
		"device_class":   "battery_charging",
		"state_topic":    stateTopic,
		"value_template": "{{ value_json.charging }}",
		"unique_id":      fmt.Sprintf("%scharging", id),
		"device": map[string]any{
			"identifiers": []string{id},
			"name":        ev.Name,
		},
	}
	if err := sendMQTT(client, chargingTopic, chargingPayload, true); err != nil {
		return err
	}

	return nil
}

func sendMQTT(client mqtt.Client, topic string, payload any, retain bool) error {
	bs, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	token := client.Publish(topic, 0, retain, bs)
	token.Wait()
	return token.Error()
}
