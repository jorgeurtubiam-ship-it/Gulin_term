# 🌐 Manual de Uso: Mapa de Servicios Gulin

Este documento explica cómo utilizar la nueva interfaz de visualización de infraestructura de Gulin Agent.

## 1. Introducción
El **Mapa de Servicios** es una herramienta de observabilidad premium diseñada para visualizar en tiempo real las conexiones, dependencias y flujos de datos entre tus servidores y servicios (Dremio, Oracle, API Gateways, etc.).

---

## 2. Métodos de Acceso
Puedes abrir la ventana del mapa de tres maneras diferentes:

### A. Comandos de Chat (Slash Commands)
Escribe cualquiera de estos comandos en el chat principal de Gulin:
- `/mapa`
- `/map`

### B. Botón en el Panel de Gulin AI
En la parte superior derecha del chat, encontrarás un icono de red (`network-wired`). Al pulsarlo, se abrirá la ventana dedicada.

### C. Botón en el Panel de Logs
Si tienes abierto el panel de **Logs de Depuración**, encontrarás un botón azul etiquetado como **"Mapa"** junto al botón de limpieza.

---

## 3. Navegación e Interacción
La ventana del mapa es totalmente interactiva:

- **Zoom**: Utiliza la rueda del ratón o gestos de zoom en el trackpad para acercarte o alejarte.
- **Pan (Arrastre)**: Haz clic izquierdo en el fondo y arrastra para moverte por el lienzo.
- **Organización de Nodos**: Puedes hacer clic y arrastrar cualquier nodo (servidor o servicio) para reorganizar la disposición visual a tu gusto.
- **Hover**: Al pasar el ratón sobre un nodo, este se iluminará para mostrar que es interactivo.

---

## 4. Indicadores Visuales
El mapa utiliza un lenguaje visual intuitivo:

| Elemento | Significado |
| :--- | :--- |
| **Líneas de Flujo Animadas** | Representan el tráfico de red activo entre servicios. |
| **Pulsos de Luz** | La velocidad del pulso indica la intensidad del tráfico (Alto/Bajo). |
| **Nodos Brillantes (Indigo)** | El servicio está en línea y respondiendo correctamente. |
| **Nodos Opacos (Naranja/Gris)** | El servicio está en estado "Pending" o desconectado. |
| **Iconos de Nodo** | Cada servicio tiene un icono único (BD, Servidor, App, Router). |

---

## 5. Notas Técnicas
Actualmente, el mapa se encuentra en **Modo Visualización**. Los nodos de **Dremio** y **Oracle 2022** están pre-configurados como marcadores de posición. 

En la siguiente fase de desarrollo, el mapa se conectará al **Motor de Descubrimiento** de Gulin para mostrar datos de red reales obtenidos vía SSH desde tus servidores pivot.

---
*GuLiN Agent - Visual Infrastructure Intelligence v1.0*
