package datalayer

import (
	"fmt"

	"a10/configuration"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var MESSAGING mqtt.Client

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("JANE: MQTT connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("JANE: MQTT connection lost. Error %v", err.Error())
}

func initialiseMessaging() {
	fmt.Println("JANE: Initialising message infrastructure MQTT connection")

	var broker = configuration.ConfigData.Messaging.Broker
	var port = configuration.ConfigData.Messaging.Port

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID(configuration.ConfigData.Messaging.ClientID)
	//opts.SetUsername("me")
	//opts.SetPassword("me2")
	//opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("JANE: Failed to initialise MQTT connection")
		panic(token.Error())
	}

	fmt.Println("JANE: Message infrastructure MQTT is running")

	MESSAGING = client
}
