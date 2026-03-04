# Plan de Implementación: Navegador Web Agéntico

Este plan detalla los pasos técnicos para convertir el navegador web interno de GuLiN en una herramienta completamente controlada por la IA, permitiendo navegación autónoma, extracción de contenido y acciones interactivas.

## Objetivos
1.  **Visibilidad**: Que la IA pueda leer el contenido de la pestaña activa.
2.  **Acción**: Que la IA pueda navegar, hacer clic y rellenar formularios.
3.  **Flujo Unificado**: Usar el panel de IA actual como centro de control.

---

## Cambios Propuestos

### Fase 1: Extracción de Contexto (Lectura)
Para que la IA sepa qué hay en la web, necesitamos "puentes" entre el proceso de Electron y el Chat.

#### [MODIFICAR] [preload-webview.ts](file:///Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/emain/preload-webview.ts)
- Añadir un escuchador IPC para solicitudes de "scraping" de texto.
- Implementar una función que limpie el HTML y devuelva solo el texto relevante (evitando scripts/estilos).

#### [MODIFICAR] [emain-ipc.ts](file:///Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/emain/emain-ipc.ts)
- Crear un nuevo handler `webview-get-text` que envíe el comando al WebView y devuelva el resultado al proceso de backend (Go).

---

### Fase 2: Herramientas Agénticas (Acción)
Añadiremos nuevas capacidades al motor de IA en el backend.

#### [NUEVO] [tools_web_extract.go](file:///Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/pkg/aiusechat/tools_web_extract.go)
- Implementar la herramienta `web_get_page_content`:
    - Recibe el `block_id` del navegador.
    - Solicita el texto al frontend vía IPC.
    - Devuelve el texto a la IA para su análisis.

#### [MODIFICAR] [tools_web.go](file:///Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/pkg/aiusechat/tools_web.go)
- Refinar la herramienta `web_navigate` (ya existente) para que sea más robusta y maneje errores de carga.

---

### Fase 3: Interfaz de Usuario (Visualización)
Mejorar el feedback para que el usuario vea qué está haciendo la IA.

#### [MODIFICAR] [aipanel.tsx](file:///Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/frontend/app/aipanel/aipanel.tsx)
- Añadir un indicador visual (ej: "Leyendo página web...") cuando la herramienta de extracción esté activa.
- Implementar banners de aprobación para acciones críticas (ej: "La IA quiere navegar a LinkedIn. ¿Permitir?").

---

## Plan de Verificación

### Pruebas Automatizadas
- Ejecutar `task dev` para validar que los nuevos canales IPC no rompen el rendimiento del terminal.
- Test de backend en Go para asegurar que la herramienta `web_navigate` resuelve correctamente los IDs de los bloques.

### Verificación Manual
1.  Abrir un bloque de navegador con una página de documentación.
2.  Preguntar en el chat: "¿De qué trata esta página?".
3.  Verificar que la IA use la nueva herramienta de extracción y responda correctamente.
4.  Pedir: "Navega a la sección de ejemplos", y verificar que la URL del navegador cambie automáticamente.
