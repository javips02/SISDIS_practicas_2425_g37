/*
* AUTOR: Rafael Tolosana Calasanz y Unai Arronategui
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2022
* FICHERO: server-draft.go
* DESCRIPCIÓN: contiene la funcionalidad esencial para realizar los servidores
*				correspondientes a la práctica 1
 */
 package main

 import (
	 "fmt"
	 "time"
	 "practica1/com"
 )
 
 // PRE: verdad = !foundDivisor
 // POST: IsPrime devuelve verdad si n es primo y falso en caso contrario
 func isPrime(n int) (foundDivisor bool) {
	 foundDivisor = false
	 for i := 2; (i < n) && !foundDivisor; i++ {
		 foundDivisor = (n%i == 0)
	 }
	 return !foundDivisor
 }
 
 // PRE: interval.A < interval.B
 // POST: FindPrimes devuelve todos los números primos comprendidos en el
 //
 //	intervalo [interval.A, interval.B]
 func findPrimes(interval com.TPInterval) (primes []int, duration time.Duration) {
	start := time.Now() // Start the timer

	for i := interval.Min; i <= interval.Max; i++ {
		if isPrime(i) {
			primes = append(primes, i)
		}
	}

	duration = time.Since(start) // Calculate elapsed time
	return primes, duration
}

 
 func main() {
	totalDuration := time.Duration(0)
	var duration time.Duration 
	for i := 0; i < 100; i++ {
		interval := com.TPInterval{Min: 1000, Max: 7000}
		_, duration = findPrimes(interval)
		totalDuration += duration
		fmt.Print("-")
	}
	totalDuration = totalDuration/100
	fmt.Println("Average over 100 runs", totalDuration)
}
 