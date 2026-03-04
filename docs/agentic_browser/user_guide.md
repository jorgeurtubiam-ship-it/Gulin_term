# Guía de Usuario: Navegador Web Agéntico en GuLiN IA

Esta guía explica cómo aprovechar las nuevas capacidades de la IA para interactuar con páginas web directamente desde tu terminal GuLiN.

## 1. ¿Cómo funciona?

Cuando tienes una pestaña de Navegador abierta en GuLiN IA, la IA detecta automáticamente este "contexto". No necesitas instalar nada adicional; el chat ahora tiene acceso a herramientas especializadas para esa pestaña.

## 2. Herramientas Disponibles

### Leer contenido (`web_read_page`)
La IA puede "ver" el texto de la página.
- **Ejemplo**: "¿Qué dice el artículo sobre Docker en esta pestaña?"
- **Resultado**: La IA leerá la página y te dará un resumen o responderá tu duda.

### Hacer clic (`web_click`)
La IA puede presionar botones o enlaces.
- **Ejemplo**: "Haz clic en el botón de 'Aceptar Cookies' y luego en 'Iniciar Sesión'".
- **Seguridad**: Aparecerá un banner en el chat preguntando: "¿Deseas permitir que la IA haga clic en `.cookie-accept`?". Debes aprobarlo para continuar.

### Escribir texto (`web_type`)
La IA puede rellenar buscadores o formularios.
- **Ejemplo**: "Busca 'Antigravity AI' en el buscador de la página".
- **Resultado**: La IA escribirá el texto y disparará los eventos necesarios para que la web lo reconozca.

## 3. Modos de Operación

- **Modo @plan**: Úsalo para investigar. La IA leerá la página y te dará un informe sin realizar acciones.
- **Modo @act**: El modo "Agente". La IA intentará resolver tareas de forma autónoma (ej: "Inicia sesión en mi cuenta"). Es más directo y requiere menos explicaciones.

## 4. Consejos de Uso y Pragmatismo (v2.0.1)

1.  **Evita interrupciones**: Cuando la IA esté ejecutando herramientas (verás iconos de carga), espera a que aparezca el botón de **Approve**. Si escribes en el chat durante ese proceso, puedes romper el flujo de la conversación (Error 400).
2.  **Órdenes Directas**: Con la nueva actualización, puedes ser muy directo: "ACT: Busca zapatillas en esta web y dime el precio de las primeras".
3.  **Habilitación de DeepSeek**: Si usas DeepSeek como proveedor, ahora tienes soporte completo para herramientas agénticas.

## 5. Solución de Problemas

- **"No encuentro el botón"**: Pide a la IA que primero use `web_read_page` para entender la estructura.
- **Error Rojo (Status 400)**: Ocurre por interrupciones en el turno de palabra. Dale a **"New Chat"** y repite la orden sin interrumpir el proceso de herramientas.
