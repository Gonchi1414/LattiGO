package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Estructura para enviar datos en texto plano (Inseguro)
type PlainRequestPayload struct {
	DataIncome float64 `json:"DataIncome"`
	DataDebt   float64 `json:"DataDebt"`
}

type PlainResponsePayload struct {
	Result float64 `json:"Result"`
}

func main() {
	fmt.Println("=== Iniciando Equipo A (Cliente MODO INSEGURO) ===")

	// Datos de prueba
	income := 456198000.0
	debt := 2000.0

	fmt.Printf("\nDatos del Cliente a enviar en CLARO:\n  Ingresos: %.2f\n  Deuda: %.2f\n", income, debt)

	// URL del servidor (Cambia localhost por la IP de la Laptop B si usas dos equipos)
	serverURL := "http://192.168.0.12:8080/evaluate-risk-plain"

	start := time.Now()

	// 1. Preparar JSON
	reqPayload := PlainRequestPayload{
		DataIncome: income,
		DataDebt:   debt,
	}
	reqBody, _ := json.Marshal(reqPayload)

	// 2. Envío de Red
	fmt.Printf("[!] Enviando datos SIN CIFRAR a %s...\n", serverURL)
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(reqBody))

	if err != nil {
		log.Fatalf("Error de conexión: %v. ¿Olvidaste agregar el endpoint al servidor?", err)
	}
	defer resp.Body.Close()

	// 3. Leer Respuesta
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("El servidor respondió con error: %s", string(body))
	}

	var resPayload PlainResponsePayload
	json.Unmarshal(body, &resPayload)

	fmt.Println("\n==========================================")
	fmt.Println("    RESULTADO DE EVALUACIÓN INSEGURA      ")
	fmt.Println("==========================================")
	fmt.Printf("Resultado recibido: %.4f\n", resPayload.Result)
	fmt.Printf("Tiempo de respuesta: %v\n", time.Since(start))
	fmt.Println("==========================================")
	fmt.Println("ALERTA: Estos datos viajaron por la red de forma legible.")
}
