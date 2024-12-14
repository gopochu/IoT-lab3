package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"

	emqxMQTT "github.com/eclipse/paho.mqtt.golang"
)

var client emqxMQTT.Client

func initMQTTClient() {
	// MQTT broker info
	broker := "tls://r112ad39.ala.eu-central-1.emqxsl.com:8883"
	username := "tgvadlap"
	password := "12345"

	// Загрузка CA сертификата
	certpool := x509.NewCertPool()
	pemCerts, err := ioutil.ReadFile("emqxsl-ca.crt")
	if err != nil {
		log.Fatal(err)
	}
	if ok := certpool.AppendCertsFromPEM(pemCerts); !ok {
		log.Fatal("Не удалось добавить CA сертификат")
	}

	// TLS конфигурация
	tlsConfig := &tls.Config{
		RootCAs: certpool,
	}

	// Опции клиента MQTT
	opts := emqxMQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("tg_mqtt_client")
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetTLSConfig(tlsConfig)

	// Создаем клиента MQTT
	client = emqxMQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	fmt.Println("Подключен к EMQX")
}

func sendData(operatingMode bool) {
	topic := "mode/topic"
	var modeMsg string
	if operatingMode {
		modeMsg = "auto"
	} else {
		modeMsg = "hand"
	}
	message := modeMsg
	if client.IsConnected() {
		token := client.Publish(topic, 0, false, message)
		token.Wait()
		fmt.Printf("Опубликовано сообщение: %s в топик: %s\n", message, topic)
	} else {
		fmt.Println("MQTT клиент не подключен")
	}
}

func subscribeToTopic(topic string, messageChannel chan<- string, isClicked bool) {
	// Подписываемся на топик
	if isClicked {
		client.Subscribe(topic, 0, func(client emqxMQTT.Client, msg emqxMQTT.Message) {
			// Отправляем полученное сообщение в канал
			messageChannel <- string(msg.Payload())
			fmt.Printf("Подписан на топик: %s\n", topic)
		})
	}
	fmt.Printf("Подписан на топик: %s\n", topic)
}
