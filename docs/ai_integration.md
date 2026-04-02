# Integración de Inteligencia Artificial y Agentes

GuLiN Agent no es solo un chat con un LLM; es un **Agente Autónomo** capaz de interactuar con su entorno mediante un conjunto de herramientas especializadas.

## GuLiN Agent vs. GuLiN Bridge

- **Agente (Local)**: Vive en tu máquina, conoce tus archivos y tiene acceso a tu terminal.
- **Bridge (Remoto/Local)**: Gestiona las credenciales de API y optimiza las llamadas a los modelos para reducir costos y latencia.

## Herramientas del Agente (Tools)

El agente tiene acceso a diversas herramientas definidas en Go (`pkg/aiusechat/tools*.go`). Las más importantes son:

| Herramienta | Descripción |
| :--- | :--- |
| `read_file` | Lee el contenido de un archivo local. |
| `write_file` | Crea o modifica archivos en el disco. |
| `ls` | Lista directorios para explorar la estructura del proyecto. |
| `terminal_run` | Ejecuta comandos en la terminal de GuLiN. |
| `web_search` | Realiza búsquedas en Google/Bing para obtener información actualizada. |
| `screenshot` | Captura la pantalla actual para análisis visual. |

## Flujo de Ejecución de una Tarea

1.  **Input del Usuario**: "Arregla el error de compilación en main.go".
2.  **Planificación**: La IA decide que primero debe leer el archivo. Envía una `tool_call: read_file`.
3.  **Ejecución Silenciosa**: El servidor Go detecta la solicitud, lee el archivo y envía el contenido de vuelta a la IA de forma invisible para el usuario.
4.  **Acción**: Tras analizar el código, la IA envía una `tool_call: write_file` con la corrección.
5.  **Verificación**: La IA puede decidir ejecutar `terminal_run: go build` para confirmar que el error desapareció.
6.  **Respuesta Final**: "He corregido el error en la línea 45 y verificado que el proyecto compila correctamente".

## Modelos de IA Soportados

Gracias a GuLiN Bridge, el agente puede alternar entre:
- **Anthropic Claude 3.5/3.7 Sonnet**: Recomendado para tareas de codificación complejas.
- **OpenAI GPT-4o / o1**: Excelente para razonamiento lógico y orquestación.
- **Google Gemini 2.0 Flash/Pro**: Ideal por su ventana de contexto masiva.
- **DeepSeek V3**: Una alternativa eficiente y de bajo costo.

## Modo Orquestador

Gulin Agent incluye un modo de **Auto-Selección de Modelo** que analiza la complejidad de la tarea y selecciona el modelo más apto (costo vs. rendimiento) para cada paso de la ejecución.
