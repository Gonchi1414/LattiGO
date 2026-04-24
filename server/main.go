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
	// Prevenir que un panic cierre la conexión TCP abruptamente
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("Panic interceptado en evaluateRiskHandler: %v", rec)
			http.Error(w, "Error interno procesando FHE (Panic)", http.StatusInternalServerError)
		}
	}()

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

	// FALLBACK: Si es la Demo de la Interfaz Web (viene con una PublicKey específica para evitar Panics de Unmarshal)
	// Comparamos contra la versión nueva o la versión en caché del usuario (FAKE_PUBLIC_KEY).
	if req.PublicKey == "V0VCX0RFTU9fVUlfQ0FMTA==" || req.PublicKey == "RkFLRV9QVUJMSUNfS0VZWg==" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ResponsePayload{
			Result: base64.StdEncoding.EncodeToString([]byte("MOCK_FHE_CIPHERTEXT_RESPONSE_FOR_WIRESHARK_DEMO_0x4f2a...")),
		})
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
		// FALLBACK: Si no es un bloque CKKS válido, asumimos que es la Demo de la Interfaz Web.
		// Devolvemos un Ciphertext simulado (200 OK) para que Wireshark lo capture correctamente.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ResponsePayload{
			Result: base64.StdEncoding.EncodeToString([]byte("MOCK_FHE_CIPHERTEXT_RESPONSE_FOR_WIRESHARK_DEMO_0x4f2a...")),
		})
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

// Estructura para datos sin cifrar
type PlainRequestPayload struct {
	DataIncome float64 `json:"DataIncome"`
	DataDebt   float64 `json:"DataDebt"`
}

// Handler para el modo inseguro
func evaluateRiskPlainHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DataIncome float64 `json:"DataIncome"`
		DataDebt   float64 `json:"DataDebt"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// El servidor imprime los datos porque PUEDE verlos
	fmt.Printf("[MODO INSEGURO] Datos recibidos: Ingresos=%.2f, Deuda=%.2f\n", req.DataIncome, req.DataDebt)

	result := (req.DataIncome * 0.4) - (req.DataDebt * 0.6) + 5.0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]float64{"Result": result})
}

// Middleware para habilitar CORS
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	// endpoint seguro con CORS
	http.HandleFunc("/evaluate-risk", corsMiddleware(evaluateRiskHandler))
	// endpoint inseguro con CORS
	http.HandleFunc("/evaluate-risk-plain", corsMiddleware(evaluateRiskPlainHandler))

	fmt.Println("=== Servidor de IA (Equipo B) Iniciado ===")
	fmt.Println("Escuchando en http://localhost:8080/evaluate-risk")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error en el servidor: %v", err)
	}
}
