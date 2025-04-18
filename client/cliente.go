package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
)

type ClientDriver struct {
	DriverNumber uint   `json:"driver_number"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	TeamName     string `json:"team_name"`
	CountryCode  string `json:"country_code"`
}

const baseURL = "http://localhost:8080/api"

func startClient() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("Menu")
		fmt.Println("1. Ver corredores")
		fmt.Println("2. Ver detalle de corredor")
		fmt.Println("3. Ver carreras")
		fmt.Println("4. Ver detalle de carrera")
		fmt.Println("5. Resumen de temporada")
		fmt.Println("6. Salir")
		fmt.Print("\nSeleccione una opción: ")

		input, _ := reader.ReadString('\n')
		opcion := strings.TrimSpace(input)

		switch opcion {
		case "1":
			verCorredores()
		case "2":
			verDetalleCorredor(reader)
		case "3":
			verCarreras()
		case "4":
			verDetalleCarrera(reader)
		case "5":
			verResumenTemporada()
		case "6":
			fmt.Println("Fin del programa!")
			return
		default:
			fmt.Println("Opción no implementada aún.")
		}
	}
}

func verCorredores() {
	resp, err := http.Get(baseURL + "/corredor")
	if err != nil {
		fmt.Println("Error al contactar el servidor:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error al obtener corredores. Código:", resp.StatusCode)
		return
	}

	var drivers []ClientDriver
	err = json.NewDecoder(resp.Body).Decode(&drivers)
	if err != nil {
		fmt.Println("Error al decodificar respuesta:", err)
		return
	}

	printDrivers(drivers)
}

func printDrivers(drivers []ClientDriver) {
	fmt.Println("-------------------------------------------------------------")
	fmt.Printf("| %-2s | %-15s | %-15s | %-8s | %-15s | %-5s |\n", "#", "Nombre", "Apellido", "N Piloto", "Equipo", "Pais")
	fmt.Println("-------------------------------------------------------------")
	for i, d := range drivers {
		fmt.Printf("| %-2d | %-15s | %-15s | %-8d | %-15s | %-5s |\n",
			i+1, d.FirstName, d.LastName, d.DriverNumber, d.TeamName, d.CountryCode)
	}
	fmt.Println("-------------------------------------------------------------")
}

func verDetalleCarrera(reader *bufio.Reader) {
	fmt.Print("Ingrese el número de la carrera (del listado): ")
	input, _ := reader.ReadString('\n')
	id := strings.TrimSpace(input)

	resp, err := http.Get(fmt.Sprintf("%s/carrera/detalle/%s", baseURL, id))
	if err != nil {
		fmt.Println("Error al contactar el servidor:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Carrera no encontrada o error del servidor.")
		return
	}

	var detalle struct {
		RaceID           int    `json:"race_id"`
		CountryName      string `json:"country_name"`
		DateStart        string `json:"date_start"`
		Year             int    `json:"year"`
		CircuitShortName string `json:"circuit_short_name"`
		Results          []struct {
			Position string `json:"position"`
			Driver   string `json:"driver"`
			Team     string `json:"team"`
			Country  string `json:"country"`
		} `json:"results"`
		FastestLap struct {
			Driver    string  `json:"driver"`
			TotalTime float64 `json:"total_time"`
			Sector1   float64 `json:"sector_1"`
			Sector2   float64 `json:"sector_2"`
			Sector3   float64 `json:"sector_3"`
		} `json:"fastest_lap"`
		MaxSpeed struct {
			Driver   string  `json:"driver"`
			SpeedKMH float64 `json:"speed_kmh"`
		} `json:"max_speed"`
	}

	err = json.NewDecoder(resp.Body).Decode(&detalle)
	if err != nil {
		fmt.Println("Error al decodificar respuesta:", err)
		return
	}

	printSessionDetail(detalle.RaceID, detalle.Results, detalle.FastestLap, detalle.MaxSpeed)
}

func printSessionDetail(raceID int, results []struct {
	Position string `json:"position"`
	Driver   string `json:"driver"`
	Team     string `json:"team"`
	Country  string `json:"country"`
}, fastestLap struct {
	Driver    string  `json:"driver"`
	TotalTime float64 `json:"total_time"`
	Sector1   float64 `json:"sector_1"`
	Sector2   float64 `json:"sector_2"`
	Sector3   float64 `json:"sector_3"`
}, maxSpeed struct {
	Driver   string  `json:"driver"`
	SpeedKMH float64 `json:"speed_kmh"`
}) {
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("| Resultados                                                  |")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("| %-10s | %-20s | %-15s | %-5s |\n", "Posición", "Piloto", "Equipo", "País")
	fmt.Println("---------------------------------------------------------------")

	// Ordenar posiciones por posición final
	sort.Slice(results, func(i, j int) bool {
		return results[i].Position < results[j].Position
	})

	// Mostrar top 3 y último
	for i, r := range results {
		if i < 3 || i == len(results)-1 {
			pos := fmt.Sprintf("%d", i+1)
			if i == len(results)-1 {
				pos = "Último"
			}
			fmt.Printf("| %-10s | %-20s | %-15s | %-5s |\n",
				pos, r.Driver, r.Team, r.Country)
		}
	}
	fmt.Println("---------------------------------------------------------------")

	// Vuelta más rápida
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("| Vuelta más rápida                                           |")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("| Piloto: %s\n", fastestLap.Driver)
	fmt.Printf("| Tiempo Total: %.3f | Sector 1: %.3f | Sector 2: %.3f | Sector 3: %.3f\n",
		fastestLap.TotalTime,
		fastestLap.Sector1,
		fastestLap.Sector2,
		fastestLap.Sector3)
	fmt.Println("---------------------------------------------------------------")

	// Velocidad máxima
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("| Velocidad máxima alcanzada                                  |")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("| Piloto: %s | Velocidad: %.1f km/h\n",
		maxSpeed.Driver, maxSpeed.SpeedKMH)
	fmt.Println("---------------------------------------------------------------")
}

func verResumenTemporada() {
	resp, err := http.Get(baseURL + "/temporada/resumen")
	if err != nil {
		fmt.Println("Error al contactar el servidor:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error al obtener resumen de temporada.")
		return
	}

	var resumen struct {
		Season      int `json:"season"`
		Top3Winners []struct {
			Position int    `json:"position"`
			Driver   string `json:"driver"`
			Team     string `json:"team"`
			Country  string `json:"country"`
			Count    int    `json:"count"`
		} `json:"top_3_winners"`
		Top3FastestLaps []struct {
			Position int    `json:"position"`
			Driver   string `json:"driver"`
			Team     string `json:"team"`
			Country  string `json:"country"`
			Count    int    `json:"count"`
		} `json:"top_3_fastest_laps"`
		Top3PolePositions []struct {
			Position int    `json:"position"`
			Driver   string `json:"driver"`
			Team     string `json:"team"`
			Country  string `json:"country"`
			Count    int    `json:"count"`
		} `json:"top_3_pole_positions"`
	}

	err = json.NewDecoder(resp.Body).Decode(&resumen)
	if err != nil {
		fmt.Println("Error al decodificar resumen:", err)
		return
	}

	printSeasonSummary(resumen.Season, resumen.Top3Winners, resumen.Top3FastestLaps, resumen.Top3PolePositions)
}

func printSeasonSummary(season int, topWinners []struct {
	Position int    `json:"position"`
	Driver   string `json:"driver"`
	Team     string `json:"team"`
	Country  string `json:"country"`
	Count    int    `json:"count"`
}, topFastestLaps []struct {
	Position int    `json:"position"`
	Driver   string `json:"driver"`
	Team     string `json:"team"`
	Country  string `json:"country"`
	Count    int    `json:"count"`
}, topPolePositions []struct {
	Position int    `json:"position"`
	Driver   string `json:"driver"`
	Team     string `json:"team"`
	Country  string `json:"country"`
	Count    int    `json:"count"`
}) {
	fmt.Printf("\n---------------------------------------------------------------------\n")
	fmt.Printf("| Top 3 Pilotos con más Victorias - Temporada %d |\n", season)
	fmt.Printf("---------------------------------------------------------------------\n")
	fmt.Printf("| %-9s | %-20s | %-10s | %-5s | %-9s |\n", "Posición", "Piloto", "Equipo", "País", "Victorias")
	for _, d := range topWinners {
		fmt.Printf("| %-9d | %-20s | %-10s | %-5s | %-9d |\n", d.Position, d.Driver, d.Team, d.Country, d.Count)
	}

	fmt.Printf("\n---------------------------------------------------------------------\n")
	fmt.Println("| Top 3 Pilotos con más Vueltas Rápidas - Temporada", season, "|")
	fmt.Printf("---------------------------------------------------------------------\n")
	fmt.Printf("| %-9s | %-20s | %-10s | %-5s | %-15s |\n", "Posición", "Piloto", "Equipo", "País", "Vueltas Rápidas")
	for _, d := range topFastestLaps {
		fmt.Printf("| %-9d | %-20s | %-10s | %-5s | %-15d |\n", d.Position, d.Driver, d.Team, d.Country, d.Count)
	}

	fmt.Printf("\n---------------------------------------------------------------------\n")
	fmt.Println("| Top 3 Pilotos con más Pole Positions - Temporada", season, "|")
	fmt.Printf("---------------------------------------------------------------------\n")
	fmt.Printf("| %-9s | %-20s | %-10s | %-5s | %-5s |\n", "Posición", "Piloto", "Equipo", "País", "Poles")
	for _, d := range topPolePositions {
		fmt.Printf("| %-9d | %-20s | %-10s | %-5s | %-5d |\n", d.Position, d.Driver, d.Team, d.Country, d.Count)
	}
}

func verCarreras() {
	resp, err := http.Get(baseURL + "/carrera")
	if err != nil {
		fmt.Println("Error al contactar el servidor:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error al obtener carreras. Código:", resp.StatusCode)
		return
	}

	var carreras []struct {
		SessionKey       int    `json:"session_key"`
		CountryName      string `json:"country_name"`
		DateStart        string `json:"date_start"`
		Year             int    `json:"year"`
		CircuitShortName string `json:"circuit_short_name"`
	}

	err = json.NewDecoder(resp.Body).Decode(&carreras)
	if err != nil {
		fmt.Println("Error al decodificar respuesta:", err)
		return
	}

	printSessions(carreras)
}

func printSessions(sessions []struct {
	SessionKey       int    `json:"session_key"`
	CountryName      string `json:"country_name"`
	DateStart        string `json:"date_start"`
	Year             int    `json:"year"`
	CircuitShortName string `json:"circuit_short_name"`
}) {
	fmt.Println("-------------------------------------------------------------------------")
	fmt.Printf("| %-2s | %-10s | %-15s | %-12s | %-6s | %-15s |\n", "#", "ID carrera", "País", "Fecha", "Year", "Circuito")
	fmt.Println("-------------------------------------------------------------------------")
	for i, s := range sessions {
		fmt.Printf("| %-2d | %-10d | %-15s | %-12s | %-6d | %-15s |\n",
			i+1, s.SessionKey, s.CountryName, s.DateStart, s.Year, s.CircuitShortName)
	}
	fmt.Println("-------------------------------------------------------------------------")
}

func verDetalleCorredor(reader *bufio.Reader) {
	fmt.Print("Ingrese el número del piloto: ")
	input, _ := reader.ReadString('\n')
	id := strings.TrimSpace(input)

	resp, err := http.Get(fmt.Sprintf("%s/corredor/detalle/%s", baseURL, id))
	if err != nil {
		fmt.Println("Error al contactar el servidor:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Piloto no encontrado o error del servidor.")
		return
	}

	var detalle struct {
		DriverID           uint `json:"driver_id"`
		PerformanceSummary struct {
			Wins     int     `json:"wins"`
			Top3     int     `json:"top_3_finishes"`
			MaxSpeed float64 `json:"max_speed"`
		} `json:"performance_summary"`
		RaceResults []struct {
			SessionKey       int     `json:"session_key"`
			CircuitShortName string  `json:"circuit_short_name"`
			Race             string  `json:"race"`
			Position         int     `json:"position"`
			FastestLap       bool    `json:"fastest_lap"`
			MaxSpeed         float64 `json:"max_speed"`
			BestLapDuration  float64 `json:"best_lap_duration"`
		} `json:"race_results"`
	}

	err = json.NewDecoder(resp.Body).Decode(&detalle)
	if err != nil {
		fmt.Println("Error al decodificar respuesta:", err)
		return
	}

	printDriverDetail(detalle.DriverID, detalle.RaceResults, detalle.PerformanceSummary)
}

func printDriverDetail(driverID uint, raceResults []struct {
	SessionKey       int     `json:"session_key"`
	CircuitShortName string  `json:"circuit_short_name"`
	Race             string  `json:"race"`
	Position         int     `json:"position"`
	FastestLap       bool    `json:"fastest_lap"`
	MaxSpeed         float64 `json:"max_speed"`
	BestLapDuration  float64 `json:"best_lap_duration"`
}, performanceSummary struct {
	Wins     int     `json:"wins"`
	Top3     int     `json:"top_3_finishes"`
	MaxSpeed float64 `json:"max_speed"`
}) {
	fmt.Println("-------------------------------------------------------------------------------------------")
	fmt.Printf("| %-2s | %-15s | %-10s | %-12s | %-12s | %-15s |\n", "#", "Carrera", "Pos Final", "Vuelta rapida", "Velocidad max", "Menor tiempo vuelta")
	fmt.Println("-------------------------------------------------------------------------------------------")

	winCount := 0
	top3Count := 0
	maxSpeed := 0.0

	for i, r := range raceResults {
		// Contar victorias y top 3
		if r.Position == 1 {
			winCount++
		}
		if r.Position <= 3 {
			top3Count++
		}

		// Actualizar velocidad máxima
		if r.MaxSpeed > maxSpeed {
			maxSpeed = r.MaxSpeed
		}

		// Formatear tiempo de vuelta
		lapTime := "N/A"
		if r.BestLapDuration > 0 {
			minutes := int(r.BestLapDuration) / 60
			seconds := r.BestLapDuration - float64(minutes*60)
			lapTime = fmt.Sprintf("%d:%06.3f", minutes, seconds)
		}

		// Formatear vuelta rápida
		fastestLap := "No"
		if r.FastestLap {
			fastestLap = "Sí"
		}

		fmt.Printf("| %-2d | %-15s | %-10d | %-12s | %-12.1f km/h | %-15s |\n",
			i+1,
			r.Race,
			r.Position,
			fastestLap,
			r.MaxSpeed,
			lapTime)
	}
	fmt.Println("-------------------------------------------------------------------------------------------")

	// Resumen del piloto
	fmt.Println("-----------------------------------------------")
	fmt.Println("| Resumen del desempeño del piloto            |")
	fmt.Println("-----------------------------------------------")
	fmt.Printf("| Carreras ganadas               | %-3d |\n", winCount)
	fmt.Printf("| Veces en el top 3              | %-3d |\n", top3Count)
	fmt.Printf("| Velocidad máxima alcanzada     | %-3.1f km/h |\n", maxSpeed)
	fmt.Println("-----------------------------------------------")
}

func main() {
	fmt.Println("Starting client...")
	startClient()
}
