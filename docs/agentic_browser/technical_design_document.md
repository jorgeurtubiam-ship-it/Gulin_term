# Diseño Técnico: Integración Navegador-Chat Agéntico

Este documento consolida la arquitectura, el flujo de datos y las herramientas implementadas para permitir la navegación agéntica dentro de GuLiN IA.

## 1. Arquitectura de Comunicación
La integración utiliza una arquitectura de puente de tres capas:
1.  **Backend (Go/AI)**: Define las herramientas agénticas y gestiona el flujo de la conversación. Utiliza el cliente RPC para comunicarse con Electron.
2.  **Proceso Principal de Electron (NodeJS)**: Actúa como mediador, traduciendo comandos RPC de Go a ejecuciones de JavaScript en el WebView.
3.  **Frontend/WebView (Preload)**: Expone capacidades de introspección (como `get-webview-text`) y permite la interacción directa con el DOM.

## 2. Herramientas Agénticas Implementadas

### 2.1 `web_read_page`
*   **Función**: Extrae el texto plano (`innerText`) de la página activa.
*   **Mecanismo**: Inyección de script vía `webContents.executeJavaScript`.
*   **Seguridad**: No ejecuta scripts de terceros; solo lectura de datos del DOM.

### 2.2 `web_click`
*   **Función**: Simula un clic en un elemento identificado por un selector CSS.
*   **Mecanismo**: Localiza el elemento en el DOM y dispara el evento `.click()`.
*   **Control**: Requiere aprobación explícita del usuario mediante el sistema de `toolapproval`.

### 2.3 `web_type`
*   **Función**: Introduce texto en campos de entrada (`input`, `textarea`).
*   **Mecanismo**: Cambia el `.value` del elemento y dispara eventos de `input` y `change` para mantener la reactividad (ej: con React/Vue en la web destino).
*   **Control**: Requiere aprobación explícita del usuario.

## 3. Seguridad y Privacidad

- **Transparencia**: El usuario siempre ve en el chat qué selector está intentando tocar la IA.
- **Aislamiento**: El navegador interno de GuLiN mantiene aislamiento de procesos.
- **Autorización**: El sistema de aprobación impide que la IA realice acciones no deseadas sin consentimiento.

## 4. Optimizaciones de Decisividad (v2.0.1)

Para mejorar la experiencia con modelos como **DeepSeek**, se han implementado las siguientes mejoras:
1.  **Habilitación de Herramientas**: Activación explícita de `AICapabilityTools` para proveedores DeepSeek en el backend.
2.  **Ruta RPC Unificada**: Corrección del direccionamiento RPC hacia el proceso principal de Electron (`electron`) para garantizar compatibilidad con comandos de navegación.
3.  **Prompts Pragmáticos**: Reducción de la verbosidad en las instrucciones del sistema para evitar errores de sincronización de mensajes (Status 400) causados por interrupciones en el flujo de herramientas.

## 5. Guía de Mantenimiento
Las definiciones de herramientas residen en `pkg/aiusechat/tools_web_*.go`.
La lógica de puente en Electron reside en `emain/emain-web.ts` y los manejadores RPC en `emain/emain-wsh.ts`.
Configuración de prompts en `pkg/aiusechat/usechat-prompts.go`.
