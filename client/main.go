package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// RequestPayload define el esquema JSON para datos cifrados
type RequestPayload struct {
	PublicKey  string `json:"PublicKey"`
	DataIncome string `json:"DataIncome"`
	DataDebt   string `json:"DataDebt"`
}

// PlainRequestPayload define el esquema JSON para texto plano
type PlainRequestPayload struct {
	DataIncome float64 `json:"DataIncome"`
	DataDebt   float64 `json:"DataDebt"`
}

// ResponsePayload define el esquema JSON de respuesta cifrada
type ResponsePayload struct {
	Result string `json:"Result"`
}

// PlainResponsePayload define el esquema JSON de respuesta en texto plano
type PlainResponsePayload struct {
	Result float64 `json:"Result"`
}

var params ckks.Parameters

func init() {
	var err error
	// Mismos parámetros exactos que el Servidor (Equipo B)
	params, err = ckks.NewParametersFromLiteral(ckks.ParametersLiteral{
		LogN:            13,
		LogQ:            []int{50, 40, 40},
		LogP:            []int{60},
		LogDefaultScale: 40,
	})
	if err != nil {
		log.Fatalf("Error inicializando parámetros: %v", err)
	}
}

func main() {
	fmt.Println("=== Iniciando Equipo A (Cliente) ===")

	// 1. Generación de Llaves
	fmt.Println("[+] Generando llaves criptográficas locales...")
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	pk := kgen.GenPublicKeyNew(sk)

	// Inicializar Herramientas Criptográficas
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, pk)
	decryptor := rlwe.NewDecryptor(params, sk)

	// Datos del cliente
	income := 5000.0
	debt := 2000.0

	fmt.Printf("\nDatos del Cliente:\n  Ingresos: %.2f\n  Deuda: %.2f\n", income, debt)

	// ============================================
	// MODO 1: Evaluación en Texto Plano (Comparativa)
	// ============================================
	evaluatePlain(income, debt)

	// ============================================
	// MODO 2: Evaluación Homomórfica
	// ============================================
	evaluateEncrypted(income, debt, encoder, encryptor, decryptor, pk)
}

func evaluateEncrypted(income, debt float64, encoder *ckks.Encoder, encryptor *rlwe.Encryptor, decryptor *rlwe.Decryptor, pk *rlwe.PublicKey) {
	fmt.Println("\n--- INICIANDO EVALUACIÓN CIFRADA (MODO SEGURO) ---")
	start := time.Now()

	// 1. Codificación y Cifrado
	incomeCt, err := encodeAndEncrypt(income, encoder, encryptor)
	if err != nil {
		log.Fatalf("Error cifrando ingresos: %v", err)
	}

	debtCt, err := encodeAndEncrypt(debt, encoder, encryptor)
	if err != nil {
		log.Fatalf("Error cifrando deuda: %v", err)
	}

	// 2. Serialización a Base64
	incomeBytes, _ := incomeCt.MarshalBinary()
	debtBytes, _ := debtCt.MarshalBinary()
	pkBytes, _ := pk.MarshalBinary()

	reqPayload := RequestPayload{
		PublicKey:  base64.StdEncoding.EncodeToString(pkBytes), // Enviamos la PublicKey por si el contrato lo requiere
		DataIncome: base64.StdEncoding.EncodeToString(incomeBytes),
		DataDebt:   base64.StdEncoding.EncodeToString(debtBytes),
	}

	reqBody, _ := json.Marshal(reqPayload)

	// 3. Envío al Servidor
	fmt.Println("[+] Enviando datos cifrados al servidor (POST /evaluate-risk)...")
	netStart := time.Now()
	resp, err := http.Post("http://localhost:8080/evaluate-risk", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Fatalf("Error conectando con el servidor (¿Está el Equipo B corriendo?): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyErr, _ := io.ReadAll(resp.Body)
		log.Fatalf("Error del servidor HTTP %d: %s", resp.StatusCode, string(bodyErr))
	}

	var respPayload ResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		log.Fatalf("Error decodificando respuesta: %v", err)
	}
	fmt.Printf("[+] Respuesta cifrada recibida del servidor en %v\n", time.Since(netStart))

	// 4. Decodificación y Deserialización de Respuesta
	resBytes, err := base64.StdEncoding.DecodeString(respPayload.Result)
	if err != nil {
		log.Fatalf("Error decodificando Base64 de la respuesta: %v", err)
	}

	resCt := rlwe.NewCiphertext(params, 1, params.MaxLevel())
	if err := resCt.UnmarshalBinary(resBytes); err != nil {
		log.Fatalf("Error deserializando el resultado cifrado: %v", err)
	}

	// 5. Descifrado
	resPlaintext := decryptor.DecryptNew(resCt)
	
	// 6. Decodificación
	output := make([]complex128, params.MaxSlots())
	if err := encoder.Decode(resPlaintext, output); err != nil {
		log.Fatalf("Error decodificando el texto plano resultante: %v", err)
	}

	resultValue := real(output[0])

	fmt.Println("\n==========================================")
	fmt.Println("   RESULTADO DE EVALUACIÓN HOMOMÓRFICA    ")
	fmt.Println("==========================================")
	fmt.Printf("Evaluación de Riesgo: %.4f\n", resultValue)
	fmt.Printf("Tiempo total (incl. red y cripto): %v\n", time.Since(start))
	fmt.Println("==========================================")
}

func encodeAndEncrypt(value float64, encoder *ckks.Encoder, encryptor *rlwe.Encryptor) (*rlwe.Ciphertext, error) {
	// Codificar un solo valor complejo en la primera ranura
	input := []complex128{complex(value, 0)}
	plaintext := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(input, plaintext); err != nil {
		return nil, err
	}
	return encryptor.EncryptNew(plaintext)
}

func evaluatePlain(income, debt float64) {
	fmt.Println("\n--- INICIANDO EVALUACIÓN EN TEXTO PLANO (MODO INSEGURO) ---")
	start := time.Now()

	reqPayload := PlainRequestPayload{
		DataIncome: income,
		DataDebt:   debt,
	}
	reqBody, _ := json.Marshal(reqPayload)

	fmt.Println("[+] Enviando datos en texto plano al servidor imaginario...")
	netStart := time.Now()
	// Intentamos pegarle a un endpoint texto plano, aunque es imaginario
	resp, err := http.Post("http://localhost:8080/evaluate-risk-plain", "application/json", bytes.NewBuffer(reqBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Servidor no tiene endpoint de texto plano (simulando localmente)...\n")
		// Simular el tiempo de latencia de red y cálculo rápido
		time.Sleep(3 * time.Millisecond)
		expectedResult := (income * 0.4) - (debt * 0.6) + 5.0
		fmt.Printf("[+] Respuesta recibida en %v\n", time.Since(netStart))
		fmt.Printf("Evaluación de Riesgo (Plana): %.4f\n", expectedResult)
	} else {
		defer resp.Body.Close()
		var respPayload PlainResponsePayload
		json.NewDecoder(resp.Body).Decode(&respPayload)
		fmt.Printf("[+] Respuesta recibida en %v\n", time.Since(netStart))
		fmt.Printf("Evaluación de Riesgo (Plana): %.4f\n", respPayload.Result)
	}
	fmt.Printf("Tiempo total (texto plano): %v\n", time.Since(start))
}
