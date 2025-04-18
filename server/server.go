package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Modelos

type Driver struct {
	DriverNumber uint   `json:"driver_number" gorm:"primaryKey"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	NameAcronym  string `json:"name_acronym"`
	TeamName     string `json:"team_name"`
	CountryCode  string `json:"country_code"`
}
type Session struct {
	SessionKey       int    `json:"session_key" gorm:"primaryKey"`
	SessionName      string `json:"session_name"`
	SessionType      string `json:"session_type"`
	Location         string `json:"location"`
	CountryName      string `json:"country_name"`
	Year             int    `json:"year"`
	CircuitShortName string `json:"circuit_short_name"`
	DateStart        string `json:"date_start"`
}
type Position struct {
	DriverNumber uint   `json:"driver_number"`
	SessionKey   int    `json:"session_key"`
	Position     int    `json:"position"`
	Date         string `json:"date"`
}

type Lap struct {
	DriverNumber    uint    `json:"driver_number"`
	SessionKey      int     `json:"session_key"`
	LapNumber       int     `json:"lap_number"`
	LapDuration     float64 `json:"lap_duration"`
	DurationSector1 float64 `json:"duration_sector_1"`
	DurationSector2 float64 `json:"duration_sector_2"`
	DurationSector3 float64 `json:"duration_sector_3"`
	StSpeed         float64 `json:"st_speed"`
	DateStart       string  `json:"date_start"`
}

// Base de datos global
var db *gorm.DB

// Inicializar DB
func initDatabase() {
	var err error
	db, err = gorm.Open(sqlite.Open("proxy.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Error conectando a proxy.db:", err)
	}

	// Migrar tabla Driver por ahora
	err = db.AutoMigrate(&Driver{}, &Session{}, &Position{}, &Lap{})
	if err != nil {
		log.Fatal("Error migrando base de datos:", err)
	}
}

// Handlers

// GET /api/corredor
func getDrivers(c *gin.Context) {
	var drivers []Driver
	result := db.Find(&drivers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener los pilotos"})
		return
	}

	// Ordenar por número de piloto
	sort.Slice(drivers, func(i, j int) bool {
		return drivers[i].DriverNumber < drivers[j].DriverNumber
	})

	// Formatear respuesta exactamente como se especifica
	var response []gin.H
	for _, d := range drivers {
		response = append(response, gin.H{
			"first_name":    d.FirstName,
			"last_name":     d.LastName,
			"driver_number": d.DriverNumber,
			"team_name":     d.TeamName,
			"country_code":  d.CountryCode,
		})
	}

	c.JSON(http.StatusOK, response)
}

func autoPopulateDriversIfNeeded() {
	var count int64
	db.Model(&Driver{}).Count(&count)

	if count > 0 {
		log.Println("✔️ Tabla de pilotos ya tiene datos.")
		return
	}

	log.Println("📥 Poblando tabla de pilotos desde OpenF1...")

	// Pilotos principales de session_key 9574
	mainDriverNums := []uint{1, 2, 3, 4, 10, 11, 14, 16, 18, 20, 22, 23, 24, 27, 31, 44, 55, 63, 77, 81}
	// Pilotos adicionales de session_key 9636
	extraDriverNums := []uint{30, 50, 43}

	var allDrivers []Driver

	// Obtener pilotos principales de session_key 9574
	mainDrivers, err := fetchSpecificDriversFromOpenF1(9574, mainDriverNums)
	if err != nil {
		log.Printf("❌ Error obteniendo pilotos principales: %v", err)
	} else {
		log.Printf("✅ Obtenidos %d pilotos principales", len(mainDrivers))
		allDrivers = append(allDrivers, mainDrivers...)
	}

	// Obtener pilotos adicionales de session_key 9636
	extraDrivers, err := fetchSpecificDriversFromOpenF1(9636, extraDriverNums)
	if err != nil {
		log.Printf("❌ Error obteniendo pilotos adicionales: %v", err)
	} else {
		log.Printf("✅ Obtenidos %d pilotos adicionales", len(extraDrivers))
		allDrivers = append(allDrivers, extraDrivers...)
	}

	// Insertar todos los pilotos en un solo batch
	if len(allDrivers) > 0 {
		result := db.CreateInBatches(allDrivers, len(allDrivers))
		if result.Error != nil {
			log.Printf("❌ Error insertando pilotos: %v", result.Error)
			return
		}
		log.Printf("✅ %d pilotos insertados en la base de datos.", len(allDrivers))
	} else {
		log.Printf("❌ No se encontraron pilotos para insertar.")
	}
}

func fetchSpecificDriversFromOpenF1(sessionKey int, driverNumbers []uint) ([]Driver, error) {
	url := fmt.Sprintf("https://api.openf1.org/v1/drivers?session_key=%d", sessionKey)

	body, err := fetchWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("error consultando OpenF1 para session %d: %v", sessionKey, err)
	}

	var allDrivers []Driver
	if err := json.Unmarshal(body, &allDrivers); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta OpenF1 para session %d: %v", sessionKey, err)
	}

	// Crear un mapa para búsqueda rápida
	targetDrivers := make(map[uint]bool)
	for _, num := range driverNumbers {
		targetDrivers[num] = true
	}

	var filteredDrivers []Driver
	for _, driver := range allDrivers {
		if targetDrivers[driver.DriverNumber] {
			filteredDrivers = append(filteredDrivers, driver)
			log.Printf("✅ Piloto encontrado: %s %s (#%d)", driver.FirstName, driver.LastName, driver.DriverNumber)
		}
	}

	if len(filteredDrivers) != len(driverNumbers) {
		// Identificar qué pilotos faltan
		missing := []uint{}
		for _, num := range driverNumbers {
			found := false
			for _, driver := range filteredDrivers {
				if driver.DriverNumber == num {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, num)
			}
		}
		log.Printf("⚠️ No se encontraron todos los pilotos. Faltan: %v", missing)
	}

	return filteredDrivers, nil
}

func getDriverDetail(c *gin.Context) {
	id := c.Param("id")

	// First try to find the driver by driver number
	var driver Driver
	if err := db.First(&driver, "driver_number = ?", id).Error; err != nil {
		// If not found by driver number, try to find by index in the list
		var drivers []Driver
		if err := db.Find(&drivers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener los pilotos"})
			return
		}

		// Convert id to int for index lookup
		index, err := strconv.Atoi(id)
		if err != nil || index < 1 || index > len(drivers) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Piloto no encontrado"})
			return
		}

		// Get the driver at the specified index (1-based)
		driver = drivers[index-1]
	}

	// Obtener todas las sesiones de una vez
	var sessions []Session
	if err := db.Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las sesiones"})
		return
	}

	// Crear un mapa de sesiones para acceso rápido
	sessionsByKey := make(map[int]Session)
	for _, s := range sessions {
		sessionsByKey[s.SessionKey] = s
	}

	// Resumen de carreras
	winCount := 0
	top3Count := 0
	maxSpeed := 0.0
	results := []gin.H{}

	// Para cada sesión, obtener los datos del piloto
	for sessionKey, session := range sessionsByKey {
		// Obtener la posición final del piloto en esta sesión
		var finalPosition Position
		if err := db.Where("session_key = ? AND driver_number = ?", sessionKey, driver.DriverNumber).
			Order("date DESC").
			First(&finalPosition).Error; err != nil {
			// Si no hay posición, saltamos esta sesión
			continue
		}

		// Obtener la mejor vuelta del piloto en esta sesión
		var bestLap Lap
		if err := db.Where("session_key = ? AND driver_number = ? AND lap_duration > 0", sessionKey, driver.DriverNumber).
			Order("lap_duration ASC").
			First(&bestLap).Error; err != nil {
			// Si no hay vuelta, usamos valores por defecto
			bestLap = Lap{
				LapDuration: 0,
				StSpeed:     0,
			}
		}

		// Obtener la vuelta más rápida de la sesión
		var fastestLapInSession Lap
		if err := db.Where("session_key = ? AND lap_duration > 0", sessionKey).
			Order("lap_duration ASC").
			First(&fastestLapInSession).Error; err != nil {
			// Si no hay vuelta más rápida, asumimos que no es la vuelta rápida
			fastestLapInSession = Lap{
				DriverNumber: 0,
				LapDuration:  0,
			}
		}

		// Actualizar contadores
		if finalPosition.Position == 1 {
			winCount++
		}
		if finalPosition.Position <= 3 {
			top3Count++
		}
		if bestLap.StSpeed > maxSpeed {
			maxSpeed = bestLap.StSpeed
		}

		// Formatear el nombre de la carrera
		raceName := fmt.Sprintf("GP de %s", session.CountryName)

		// Agregar resultado
		results = append(results, gin.H{
			"session_key":        sessionKey,
			"circuit_short_name": session.CircuitShortName,
			"race":               raceName,
			"position":           finalPosition.Position,
			"fastest_lap":        bestLap.DriverNumber == fastestLapInSession.DriverNumber && bestLap.LapDuration > 0,
			"max_speed":          bestLap.StSpeed,
			"best_lap_duration":  bestLap.LapDuration,
		})
	}

	// Ordenar resultados por fecha de sesión
	sort.Slice(results, func(i, j int) bool {
		return sessionsByKey[results[i]["session_key"].(int)].DateStart < sessionsByKey[results[j]["session_key"].(int)].DateStart
	})

	c.JSON(http.StatusOK, gin.H{
		"driver_id": driver.DriverNumber,
		"performance_summary": gin.H{
			"wins":           winCount,
			"top_3_finishes": top3Count,
			"max_speed":      maxSpeed,
		},
		"race_results": results,
	})
}

// Función auxiliar para hacer peticiones HTTP con reintentos
func fetchWithRetry(url string, maxRetries int) ([]byte, error) {
	var resp *http.Response
	var err error
	backoff := time.Second

	log.Printf("📡 Consultando URL: %s", url)

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			log.Printf("🔄 Reintento %d/%d para URL: %s", i+1, maxRetries, url)
		}

		resp, err = http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("❌ Error leyendo respuesta: %v", err)
				if i < maxRetries-1 {
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
				return nil, err
			}

			// Verificar que la respuesta no esté vacía
			if len(body) == 0 {
				log.Printf("⚠️ Respuesta vacía recibida")
				if i < maxRetries-1 {
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
				return nil, fmt.Errorf("respuesta vacía de la API")
			}

			// Verificar que la respuesta sea un JSON válido
			if !json.Valid(body) {
				log.Printf("⚠️ Respuesta no es JSON válido")
				if i < maxRetries-1 {
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
				return nil, fmt.Errorf("respuesta no es JSON válido")
			}

			// Verificar que sea un array
			var result interface{}
			if err := json.Unmarshal(body, &result); err != nil {
				log.Printf("⚠️ Error al deserializar JSON: %v", err)
				if i < maxRetries-1 {
					time.Sleep(backoff)
					backoff *= 2
					continue
				}
				return nil, fmt.Errorf("respuesta no es un JSON válido: %v", err)
			}

			// Verificar el tipo de resultado
			switch v := result.(type) {
			case []interface{}:
				if len(v) == 0 {
					log.Printf("⚠️ Array JSON vacío recibido")
					if i < maxRetries-1 {
						time.Sleep(backoff)
						backoff *= 2
						continue
					}
					return nil, fmt.Errorf("array JSON vacío")
				}
			case map[string]interface{}:
				if len(v) == 0 {
					log.Printf("⚠️ Objeto JSON vacío recibido")
					if i < maxRetries-1 {
						time.Sleep(backoff)
						backoff *= 2
						continue
					}
					return nil, fmt.Errorf("objeto JSON vacío")
				}
			}

			log.Printf("✅ Respuesta válida recibida (%d bytes)", len(body))
			return body, nil
		}

		// Manejar errores HTTP
		if resp != nil {
			log.Printf("❌ Error HTTP %d recibido", resp.StatusCode)
			resp.Body.Close()
		} else {
			log.Printf("❌ Error de conexión: %v", err)
		}

		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("error después de %d intentos: %v", maxRetries, err)
}

func autoPopulateSessionsIfNeeded() {
	var count int64
	db.Model(&Session{}).Count(&count)

	if count > 0 {
		log.Println("✔️ Tabla de carreras ya tiene datos.")
		return
	}

	log.Println("📥 Poblando tabla de carreras desde OpenF1...")

	url := "https://api.openf1.org/v1/sessions?session_name=Race&year=2024"
	log.Printf("🔍 Consultando carreras de 2024: %s", url)

	body, err := fetchWithRetry(url, 3)
	if err != nil {
		log.Fatalf("❌ Error consultando sesiones: %v", err)
	}

	var sessions []Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		log.Fatalf("❌ Error parseando sesiones: %v", err)
	}

	if len(sessions) == 0 {
		log.Fatalf("❌ No se encontraron carreras para el año 2024")
	}

	log.Printf("✅ Obtenidas %d carreras para el año 2024", len(sessions))

	// Mostrar información de las carreras encontradas
	for _, s := range sessions {
		log.Printf("🏁 Carrera: %s en %s (%s) - session_key: %d",
			s.SessionName, s.CountryName, s.DateStart, s.SessionKey)
	}

	// Insertar todas las sesiones en un solo batch
	if result := db.CreateInBatches(sessions, len(sessions)); result.Error != nil {
		log.Fatalf("❌ Error insertando carreras: %v", result.Error)
	}

	log.Printf("✅ %d carreras insertadas en la base de datos.", len(sessions))
}

func autoPopulatePositionsAndLapsIfNeeded() {
	var posCount, lapCount int64
	db.Model(&Position{}).Count(&posCount)
	db.Model(&Lap{}).Count(&lapCount)

	if posCount > 0 && lapCount > 0 {
		log.Println("✔️ Tablas de posiciones y vueltas ya tienen datos.")
		return
	}

	log.Println("📥 Poblando tablas de posiciones y vueltas desde OpenF1...")

	// Obtener todas las sesiones
	var sessions []Session
	if err := db.Find(&sessions).Error; err != nil {
		log.Printf("❌ Error obteniendo sesiones: %v", err)
		return
	}

	if len(sessions) == 0 {
		log.Printf("❌ No se encontraron sesiones para obtener posiciones y vueltas.")
		return
	}

	log.Printf("🔍 Encontradas %d sesiones para procesar", len(sessions))

	// Semáforo para limitar peticiones concurrentes
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	// Variables para almacenar resultados
	var mu sync.Mutex
	allPositions := make([]Position, 0)
	allLaps := make([]Lap, 0)

	// Procesar cada sesión
	for _, session := range sessions {
		wg.Add(1)
		go func(s Session) {
			defer wg.Done()
			sem <- struct{}{}        // Adquirir semáforo
			defer func() { <-sem }() // Liberar semáforo

			log.Printf("🔄 Procesando sesión %d: %s en %s", s.SessionKey, s.SessionName, s.CountryName)

			// 1. Obtener posiciones
			positions, err := fetchPositionsFromAPI(s.SessionKey)
			if err != nil {
				log.Printf("❌ Error obteniendo posiciones para sesión %d: %v", s.SessionKey, err)
			} else {
				log.Printf("✅ Obtenidas %d posiciones para sesión %d", len(positions), s.SessionKey)
				mu.Lock()
				allPositions = append(allPositions, positions...)
				mu.Unlock()
			}

			// 2. Obtener vueltas
			laps, err := fetchLapsFromAPI(s.SessionKey)
			if err != nil {
				log.Printf("❌ Error obteniendo vueltas para sesión %d: %v", s.SessionKey, err)
			} else {
				log.Printf("✅ Obtenidas %d vueltas para sesión %d", len(laps), s.SessionKey)
				mu.Lock()
				allLaps = append(allLaps, laps...)
				mu.Unlock()
			}
		}(session)
	}

	// Esperar a que todas las goroutines terminen
	wg.Wait()

	log.Printf("📊 Resultados finales: %d posiciones y %d vueltas obtenidas", len(allPositions), len(allLaps))

	// Insertar posiciones en la base de datos
	if len(allPositions) > 0 {
		batchSize := 1000
		var insertedPos int
		for i := 0; i < len(allPositions); i += batchSize {
			end := i + batchSize
			if end > len(allPositions) {
				end = len(allPositions)
			}
			batch := allPositions[i:end]

			if result := db.CreateInBatches(batch, len(batch)); result.Error != nil {
				log.Printf("❌ Error insertando lote de posiciones %d-%d: %v", i, end, result.Error)
			} else {
				insertedPos += int(result.RowsAffected)
				log.Printf("✅ Insertadas %d posiciones (lote %d-%d)", result.RowsAffected, i, end)
			}
		}
		log.Printf("✅ Total de %d posiciones insertadas en la base de datos", insertedPos)
	} else {
		log.Printf("⚠️ No se encontraron posiciones para insertar")
	}

	// Insertar vueltas en la base de datos
	if len(allLaps) > 0 {
		batchSize := 1000
		var insertedLaps int
		for i := 0; i < len(allLaps); i += batchSize {
			end := i + batchSize
			if end > len(allLaps) {
				end = len(allLaps)
			}
			batch := allLaps[i:end]

			if result := db.CreateInBatches(batch, len(batch)); result.Error != nil {
				log.Printf("❌ Error insertando lote de vueltas %d-%d: %v", i, end, result.Error)
			} else {
				insertedLaps += int(result.RowsAffected)
				log.Printf("✅ Insertadas %d vueltas (lote %d-%d)", result.RowsAffected, i, end)
			}
		}
		log.Printf("✅ Total de %d vueltas insertadas en la base de datos", insertedLaps)
	} else {
		log.Printf("⚠️ No se encontraron vueltas para insertar")
	}
}

func fetchPositionsFromAPI(sessionKey int) ([]Position, error) {
	url := fmt.Sprintf("https://api.openf1.org/v1/position?session_key=%d", sessionKey)

	log.Printf("🔍 Consultando posiciones de sesión %d: %s", sessionKey, url)

	body, err := fetchWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("error consultando posiciones: %v", err)
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("error parseando posiciones: %v", err)
	}

	// Asegurarse de que todas las posiciones tengan el session_key correcto
	for i := range positions {
		positions[i].SessionKey = sessionKey
	}

	return positions, nil
}

func fetchLapsFromAPI(sessionKey int) ([]Lap, error) {
	url := fmt.Sprintf("https://api.openf1.org/v1/laps?session_key=%d", sessionKey)

	log.Printf("🔍 Consultando vueltas de sesión %d: %s", sessionKey, url)

	body, err := fetchWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("error consultando vueltas: %v", err)
	}

	var laps []Lap
	if err := json.Unmarshal(body, &laps); err != nil {
		return nil, fmt.Errorf("error parseando vueltas: %v", err)
	}

	// Asegurarse de que todas las vueltas tengan el session_key correcto
	for i := range laps {
		laps[i].SessionKey = sessionKey
	}

	return laps, nil
}

func getSessions(c *gin.Context) {
	var sessions []Session
	result := db.Find(&sessions)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las carreras"})
		return
	}

	var carreras []gin.H
	for _, s := range sessions {
		carreras = append(carreras, gin.H{
			"session_key":        s.SessionKey,
			"country_name":       s.CountryName,
			"date_start":         s.DateStart,
			"year":               s.Year,
			"circuit_short_name": s.CircuitShortName,
		})
	}

	c.JSON(http.StatusOK, carreras)
}

func getSessionDetail(c *gin.Context) {
	id := c.Param("id")

	// Convert id to int for session_key lookup
	sessionKey, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de carrera inválido"})
		return
	}

	// Find session by session_key
	var session Session
	if err := db.First(&session, "session_key = ?", sessionKey).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Carrera no encontrada"})
		return
	}

	// POSICIONES
	var positions []Position
	if err := db.Where("session_key = ?", session.SessionKey).Order("position ASC").Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las posiciones"})
		return
	}

	resultados := []gin.H{}
	ultimo := Position{}
	if len(positions) > 0 {
		ultimo = positions[len(positions)-1]
	}

	// Mapa para rastrear pilotos ya mostrados
	pilotosMostrados := make(map[uint]bool)

	// Primero mostrar top 3
	for i := 0; i < 3 && i < len(positions); i++ {
		p := positions[i]
		var driver Driver
		if err := db.First(&driver, "driver_number = ?", p.DriverNumber).Error; err != nil {
			log.Printf("Error obteniendo piloto: %v", err)
			continue
		}
		resultados = append(resultados, gin.H{
			"position": fmt.Sprintf("%d", p.Position),
			"driver":   fmt.Sprintf("%s %s", driver.FirstName, driver.LastName),
			"team":     driver.TeamName,
			"country":  driver.CountryCode,
		})
		pilotosMostrados[p.DriverNumber] = true
	}

	// Luego mostrar último si no está en el top 3
	if !pilotosMostrados[ultimo.DriverNumber] {
		var driver Driver
		if err := db.First(&driver, "driver_number = ?", ultimo.DriverNumber).Error; err != nil {
			log.Printf("Error obteniendo piloto: %v", err)
		} else {
			resultados = append(resultados, gin.H{
				"position": "Último",
				"driver":   fmt.Sprintf("%s %s", driver.FirstName, driver.LastName),
				"team":     driver.TeamName,
				"country":  driver.CountryCode,
			})
		}
	}

	// VUELTA RÁPIDA - Obtener la vuelta más rápida con todos sus sectores
	var fastest Lap
	if err := db.Where("session_key = ? AND lap_duration > 0", session.SessionKey).
		Order("lap_duration ASC").
		First(&fastest).Error; err != nil {
		log.Printf("Error obteniendo vuelta rápida: %v", err)
		// Usar valores por defecto si no hay vuelta rápida
		fastest = Lap{
			LapDuration:     0,
			DurationSector1: 0,
			DurationSector2: 0,
			DurationSector3: 0,
		}
	}

	var fastDriver Driver
	if fastest.DriverNumber > 0 {
		if err := db.First(&fastDriver, "driver_number = ?", fastest.DriverNumber).Error; err != nil {
			log.Printf("Error obteniendo piloto de vuelta rápida: %v", err)
		}
	}

	// VELOCIDAD MÁXIMA
	var speedLap Lap
	if err := db.Where("session_key = ? AND st_speed > 0", session.SessionKey).
		Order("st_speed DESC").
		First(&speedLap).Error; err != nil {
		log.Printf("Error obteniendo velocidad máxima: %v", err)
		// Usar valores por defecto si no hay velocidad máxima
		speedLap = Lap{
			StSpeed: 0,
		}
	}

	var speedDriver Driver
	if speedLap.DriverNumber > 0 {
		if err := db.First(&speedDriver, "driver_number = ?", speedLap.DriverNumber).Error; err != nil {
			log.Printf("Error obteniendo piloto de velocidad máxima: %v", err)
		}
	}

	// RESPUESTA
	c.JSON(http.StatusOK, gin.H{
		"race_id":            session.SessionKey,
		"country_name":       session.CountryName,
		"date_start":         session.DateStart,
		"year":               session.Year,
		"circuit_short_name": session.CircuitShortName,
		"results":            resultados,
		"fastest_lap": gin.H{
			"driver":     fmt.Sprintf("%s %s", fastDriver.FirstName, fastDriver.LastName),
			"total_time": fastest.LapDuration,
			"sector_1":   fastest.DurationSector1,
			"sector_2":   fastest.DurationSector2,
			"sector_3":   fastest.DurationSector3,
		},
		"max_speed": gin.H{
			"driver":    fmt.Sprintf("%s %s", speedDriver.FirstName, speedDriver.LastName),
			"speed_kmh": speedLap.StSpeed,
		},
	})
}

func getSeasonSummary(c *gin.Context) {
	type Count struct {
		DriverNumber uint
		Count        int
	}

	// === VICTORIAS ===
	var wins []Count
	db.Raw(`
		SELECT driver_number, COUNT(*) as count
		FROM positions
		WHERE position = 1
		GROUP BY driver_number
		ORDER BY count DESC
		LIMIT 3
	`).Scan(&wins)

	// === VUELTAS RÁPIDAS ===
	var fastLaps []Count
	db.Raw(`
		SELECT driver_number, COUNT(*) as count
		FROM (
			SELECT driver_number, session_key, MIN(lap_duration)
			FROM laps
			GROUP BY session_key
		)
		GROUP BY driver_number
		ORDER BY count DESC
		LIMIT 3
	`).Scan(&fastLaps)

	// === POLES === (posición 1 en la primera fecha de cada sesión)
	var poles []Count
	db.Raw(`
		SELECT driver_number, COUNT(*) as count
		FROM positions
		WHERE position = 1
		GROUP BY driver_number
		ORDER BY count DESC
		LIMIT 3
	`).Scan(&poles)

	// Utilidad para formatear respuesta
	format := func(cs []Count) []gin.H {
		var result []gin.H
		for i, c := range cs {
			var d Driver
			db.First(&d, "driver_number = ?", c.DriverNumber)
			result = append(result, gin.H{
				"position": i + 1,
				"driver":   fmt.Sprintf("%s %s", d.FirstName, d.LastName),
				"team":     d.TeamName,
				"country":  d.CountryCode,
				"count":    c.Count,
			})
		}
		return result
	}

	c.JSON(http.StatusOK, gin.H{
		"season":               2024,
		"top_3_winners":        format(wins),
		"top_3_fastest_laps":   format(fastLaps),
		"top_3_pole_positions": format(poles),
	})
}

func getDriverPositions(c *gin.Context) {
	id := c.Param("id")

	// First try to find the driver by driver number
	var driver Driver
	if err := db.First(&driver, "driver_number = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Piloto no encontrado"})
		return
	}

	// Obtener todas las posiciones del piloto
	var positions []Position
	if err := db.Where("driver_number = ?", driver.DriverNumber).Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las posiciones"})
		return
	}

	// Obtener todas las sesiones de una vez
	var sessions []Session
	if err := db.Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las sesiones"})
		return
	}

	// Crear un mapa de sesiones para acceso rápido
	sessionsByKey := make(map[int]Session)
	for _, s := range sessions {
		sessionsByKey[s.SessionKey] = s
	}

	// Formatear la respuesta
	response := gin.H{
		"driver_id": driver.DriverNumber,
		"positions": []gin.H{},
	}

	// Para cada posición, obtener los datos de la sesión
	for _, pos := range positions {
		session, exists := sessionsByKey[pos.SessionKey]
		if !exists {
			continue
		}

		// Formatear el nombre de la carrera
		raceName := fmt.Sprintf("GP de %s", session.CountryName)

		// Agregar posición
		response["positions"] = append(response["positions"].([]gin.H), gin.H{
			"session_key":        pos.SessionKey,
			"circuit_short_name": session.CircuitShortName,
			"race":               raceName,
			"position":           pos.Position,
			"date":               pos.Date,
		})
	}

	// Ordenar posiciones por fecha
	sort.Slice(response["positions"].([]gin.H), func(i, j int) bool {
		return response["positions"].([]gin.H)[i]["date"].(time.Time).Before(response["positions"].([]gin.H)[j]["date"].(time.Time))
	})

	c.JSON(http.StatusOK, response)
}

func startServer() {
	initDatabase()

	autoPopulateDriversIfNeeded()
	autoPopulateSessionsIfNeeded()
	autoPopulatePositionsAndLapsIfNeeded()

	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/corredor", getDrivers)
		api.GET("/carrera", getSessions)
		api.GET("/corredor/detalle/:id", getDriverDetail)
		api.GET("/carrera/detalle/:id", getSessionDetail)
		api.GET("/temporada/resumen", getSeasonSummary)
		api.GET("/corredor/posiciones/:id", getDriverPositions)
	}

	if err := r.Run(":8080"); err != nil {
		log.Fatal("No se pudo iniciar el servidor:", err)
	}
}

func main() {
	fmt.Println("Starting server...")
	startServer()
}
