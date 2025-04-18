# F1 StatsHub System

Sistema de estadísticas de Fórmula 1 desarrollado como parte de la tarea para el curso de Sistemas Distribuidos.

## Descripción

Este sistema permite consultar estadísticas de la temporada 2024 de Fórmula 1, incluyendo:
- Información de pilotos
- Detalles de carreras
- Estadísticas de vueltas
- Posiciones
- Resumen de temporada

## Estructura

El sistema consta de dos componentes principales:
- **Servidor (API)**: Implementado en Go con Gin Framework y SQLite
- **Cliente**: Aplicación de línea de comandos para interactuar con la API

## Instrucciones de uso

### Requisitos
- Go 1.16 o superior
- SQLite

### Ejecución

1. Iniciar el servidor:
```
go run server/server.go
```

2. En otra terminal, iniciar el cliente:
```
go run client/cliente.go
```

3. Seguir las instrucciones del menú para interactuar con el sistema.

## Características

- Consulta de pilotos y sus estadísticas
- Visualización de detalles de carreras
- Resumen de temporada con estadísticas globales
- Almacenamiento local de datos mediante SQLite
- Sincronización con la API de OpenF1

## Endpoints API

- `/api/corredor`: Lista de pilotos
- `/api/corredor/detalle/{id}`: Detalles de un piloto específico
- `/api/carrera`: Lista de carreras
- `/api/carrera/detalle/{id}`: Detalles de una carrera específica
- `/api/temporada/resumen`: Resumen de la temporada
- `/api/corredor/posiciones/{id}`: Posiciones de un piloto específico 