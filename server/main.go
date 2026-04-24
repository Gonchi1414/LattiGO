package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// RequestPayload define la estructura de la petición JSON
type RequestPayload struct {
	PublicKey  string `json:"PublicKey"`
	DataIncome string `json:"DataIncome"`
	DataDebt   string `json:"DataDebt"`
}

// ResponsePayload define la estructura de la respuesta JSON
type ResponsePayload struct {
	Result string `json:"Result"`
}

// params almacena los parámetros criptográficos globales
var params ckks.Parameters

func init() {
	var err error
	// Parámetros estándar: CKKS con LogN: 13
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

// evaluateRiskHandler procesa el riesgo calculando: (Income * 0.4) - (Debt * 0.6) + 5.0
func evaluateRiskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decodificar JSON de entrada
	var req RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error decodificando JSON", http.StatusBadRequest)
		return
	}

	// 2. Decodificar Base64
	incomeBytes, err := base64.StdEncoding.DecodeString(req.DataIncome)
	if err != nil {
		http.Error(w, "Error decodificando DataIncome desde Base64", http.StatusBadRequest)
		return
	}

	debtBytes, err := base64.StdEncoding.DecodeString(req.DataDebt)
	if err != nil {
		http.Error(w, "Error decodificando DataDebt desde Base64", http.StatusBadRequest)
		return
	}

	// 3. Reconstruir objetos (UnmarshalBinary)
	incomeCt := rlwe.NewCiphertext(params, 1, params.MaxLevel())
	if err := incomeCt.UnmarshalBinary(incomeBytes); err != nil {
		http.Error(w, fmt.Sprintf("Error deserializando DataIncome: %v", err), http.StatusInternalServerError)
		return
	}

	debtCt := rlwe.NewCiphertext(params, 1, params.MaxLevel())
	if err := debtCt.UnmarshalBinary(debtBytes); err != nil {
		http.Error(w, fmt.Sprintf("Error deserializando DataDebt: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Inicializar Evaluator sin EvaluationKey (solo multiplicamos por escalares públicos)
	evaluator := ckks.NewEvaluator(params, nil)

	// 5. Lógica del Modelo
	// Paso A: (Income * 0.4)
	incomeScaled, err := evaluator.MulNew(incomeCt, 0.4)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error multiplicando Income: %v", err), http.StatusInternalServerError)
		return
	}

	// Paso B: (Debt * 0.6)
	debtScaled, err := evaluator.MulNew(debtCt, 0.6)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error multiplicando Debt: %v", err), http.StatusInternalServerError)
		return
	}

	// Paso C: (Income * 0.4) - (Debt * 0.6)
	resSub, err := evaluator.SubNew(incomeScaled, debtScaled)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error restando Debt a Income: %v", err), http.StatusInternalServerError)
		return
	}

	// Paso D: resSub + 5.0
	resFinal, err := evaluator.AddNew(resSub, 5.0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error sumando la constante 5.0: %v", err), http.StatusInternalServerError)
		return
	}

	// 6. Serializar resultado a Base64
	resBytes, err := resFinal.MarshalBinary()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error serializando el resultado: %v", err), http.StatusInternalServerError)
		return
	}

	resp := ResponsePayload{
		Result: base64.StdEncoding.EncodeToString(resBytes),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Error codificando respuesta JSON", http.StatusInternalServerError)
		return
	}
}

func main() {
	// Configurar endpoint
	http.HandleFunc("/evaluate-risk", evaluateRiskHandler)

	fmt.Println("=== Servidor de IA (Equipo B) Iniciado ===")
	fmt.Println("Escuchando en http://localhost:8080/evaluate-risk")
	
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error en el servidor: %v", err)
	}
}
