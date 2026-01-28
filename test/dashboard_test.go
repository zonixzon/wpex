package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	fmt.Println("🧪 Testing WPEX Statistics Dashboard...")
	
	// Aspetta che il server sia pronto
	time.Sleep(2 * time.Second)
	
	fmt.Println("\n📊 Testing API Endpoints...")
	
	// Test Health endpoint
	fmt.Println("\n🔍 Testing /health endpoint:")
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("✅ Health Status: %s\n", string(body))
	
	// Test Stats endpoint
	fmt.Println("\n🔍 Testing /stats endpoint:")
	resp, err = http.Get("http://localhost:8080/stats")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, _ = io.ReadAll(resp.Body)
	fmt.Printf("✅ Stats JSON Length: %d bytes\n", len(body))
	
	// Mostra i primi 300 caratteri del JSON
	if len(body) > 300 {
		fmt.Printf("📋 Stats Preview: %s...\n", string(body[:300]))
	} else {
		fmt.Printf("📋 Stats Content: %s\n", string(body))
	}
	
	// Test Dashboard endpoint
	fmt.Println("\n🔍 Testing / (dashboard) endpoint:")
	resp, err = http.Get("http://localhost:8080/")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, _ = io.ReadAll(resp.Body)
	fmt.Printf("✅ Dashboard HTML Length: %d bytes\n", len(body))
	
	// Verifica che non ci siano più errori di formattazione
	htmlContent := string(body)
	if bytes.Contains(body, []byte("%!")) {
		fmt.Println("❌ Found formatting errors in HTML!")
	} else {
		fmt.Println("✅ No formatting errors found in HTML!")
	}
	
	// Controlla se ci sono i valori corretti
	if bytes.Contains(body, []byte("Connected Peers")) {
		fmt.Println("✅ Dashboard contains peer information")
	}
	
	if bytes.Contains(body, []byte("Server Uptime")) {
		fmt.Println("✅ Dashboard contains uptime information") 
	}
	
	fmt.Println("\n🌐 Dashboard URLs:")
	fmt.Println("   Main Dashboard: http://localhost:8080/")
	fmt.Println("   JSON API:       http://localhost:8080/stats")
	fmt.Println("   Health Check:   http://localhost:8080/health")
	
	fmt.Println("\n✅ All tests completed!")
}