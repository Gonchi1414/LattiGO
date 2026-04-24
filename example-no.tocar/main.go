package main

import (
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	// 1. Parámetros
	params, err := ckks.NewParametersFromLiteral(ckks.ParametersLiteral{
		LogN:            13,
		LogQ:            []int{50, 40, 40},
		LogP:            []int{60},
		LogDefaultScale: 40,
	})
	if err != nil {
		panic(err)
	}

	// 2. Generación de llaves (En v6 se pasan punteros a los objetos de destino, o se usa el sufijo New)
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	pk := kgen.GenPublicKeyNew(sk)

	// 3. Inicializar herramientas
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, pk)
	decryptor := rlwe.NewDecryptor(params, sk)
	evaluator := ckks.NewEvaluator(params, nil)

	// 4. Datos de entrada
	input := []complex128{3.14159265, 0, 0, 0}

	// Cifrado (En v6, EncryptNew devuelve (ciphertext, error))
	plaintext := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(input, plaintext); err != nil {
		panic(err)
	}

	ciphertext, err := encryptor.EncryptNew(plaintext)
	if err != nil {
		panic(err)
	}

	// 5. Operación: Multiplicar por una constante
	// En v6 se usa el método Mul y acepta escalares como operandos
	if err := evaluator.Mul(ciphertext, 2.0, ciphertext); err != nil {
		panic(err)
	}

	// 6. Descifrado y Decodificación
	resPlaintext := decryptor.DecryptNew(ciphertext)
	output := make([]complex128, params.MaxSlots())
	if err := encoder.Decode(resPlaintext, output); err != nil {
		panic(err)
	}

	// Resultado
	fmt.Printf("Ciframos 3.1415 y lo multiplicamos por 2...\n")
	fmt.Printf("Resultado descifrado: %.8f\n", real(output[0]))
}
