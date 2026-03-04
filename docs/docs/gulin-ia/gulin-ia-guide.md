# 📖 Gulin IA - Guía de Usuario y Funcionalidades Avanzadas
**Asistente Inteligente de Terminal y Desarrollo con Contexto Total**

Bienvenido a **Gulin IA**, tu copiloto de ingeniería de software directamente integrado en tu terminal. Gulin no es solo un chat de inteligencia artificial; es un asistente consciente de tu entorno de trabajo que entiende tus archivos, comandos, errores y la arquitectura de tus proyectos en tiempo real.

A continuación, se detalla todo lo que puedes hacer con Gulin y cómo maximizar su potencial.

---

## 🚀 1. Capacidades Core (¿Qué hace a Gulin diferente?)

### 👁️ Contexto Absoluto (Widget Context)
Gulin ve lo mismo que tú. Gracias al switch de **"Widget Context ON"** (esquina superior derecha), Gulin lee automáticamente:
*   **La ruta de tu proyecto:** Sabe en qué carpeta estás trabajando en tiempo real.
*   **Archivos en pantalla:** Si tienes el código de `app.js` o `backend.go` abierto en Wave Terminal, Gulin lo está analizando.
*   **La salida de la terminal (Logs y Errores):** Si ejecutas un comando y falla, no necesitas copiar y pegar el error de vuelta al chat. Gulin ya lo leyó de la terminal adyacente y puede darte la solución o explicarte el *stacktrace* al instante.

### 🧠 Memoria a Largo Plazo (Gulin Brain)
A diferencia de otros asistentes web que olvidan el contexto al cerrar la pestaña o recargar la página, Gulin cuenta con un módulo interno de **Memoria a Largo Plazo (Brain)** que guarda tus **decisiones de arquitectura, hábitos de código y reglas del proyecto**.
*   **¿Cómo funciona?** El sistema guarda vectores de conocimiento en archivos locales (ej. `gulin/*.md`) asociados al proyecto. 
*   **Ejemplo de uso:** Al inicio de un proyecto, puedes decirle a Gulin: *"En este proyecto de React siempre usamos Tailwind, Axios, y toda la lógica va en la carpeta /services"*. Gulin indexará esta regla. Semanas después, en un chat totalmente nuevo de ese mismo proyecto, si le pides crear un componente, lo hará usando Tailwind y guardará la lógica en `/services` sin que tú tengas que repetirle las instrucciones de estilo.
*   **Gestión Visual:** Puedes revisar qué sabe Gulin de tu proyecto en el panel *Gulin Dashboard* (Memoria y Chats).

### 🔌 Modelos Flexibles (Local y Nube)
Gulin soporta múltiples motores de inteligencia artificial para adaptarse a tu nivel de privacidad y necesidad de cómputo:
*   **Nube (DeepSeek, Claude, GPT):** Para tareas complejas de razonamiento estructural, integrando APIs líderes en la industria.
*   **Local Automático (Ollama/Llama3):** ¿Trabajas con código propietario bajo estricto NDA? Gulin puede operar **100% offline** y de forma privada conectándose a modelos instalados en tu máquina (ej. Llama 3). Nada de tu código sale de tu ordenador.

---

## 🛠️ 2. Flujo de Trabajo y Herramientas Delegadas (Tools)

Gulin no es un simple asesor de texto; es un agente que **actúa** sobre tu máquina cuando se lo permites. Mediante el modo **ACT** o herramientas aprobadas, Gulin puede ejecutar acciones complejas:

1.  **Exploración Profunda (`readdir` / `search` / `readfile`):** Si te enfrentas a una base de código enorme y desconocida, puedes pedirle a Gulin: *"Encuentra dónde manejamos los tokens de sesión de Firebase y léeme ese código"*. Gulin rastreará el directorio completo, abrirá los archivos internamente y te resumirá la lógica en segundos.
2.  **Ejecución y Control de Terminal Automático (`term`):** 
    Gulin puede escribir, preparar y (con tu aprobación explícita) **ejecutar comandos reales** en tu terminal. 
    *   *Ejemplo:* *"Revisa qué puerto tiene ocupado el backend y mátalo, luego instala los nuevos paquetes de npm y vuelve a levantar la aplicación en modo watch"*. Gulin generará el script de bash exacto y esperará a que des clic en "Aprobar" para ejecutarlo secuencialmente.
3.  **Generación de Proyectos en Lote (Builder / Tsunami):** 
    Para las fases iniciales, Gulin puede crear la estructura de un proyecto entero, incluyendo archivos `CMakeLists`, `package.json`, configuraciones de Docker y el código base (boilerplate) escribiendo o editando múltiples archivos a la vez en el disco duro.
4.  **Análisis Multimodal (Imágenes y PDFs):** Gulin soporta *Drag & Drop*. Puedes arrastrar diagramas de arquitectura (PNG, JPG) o capturas de un error de interfaz (UI), y Gulin le pedirá al modelo de visión que lo analice y genere el código CSS/HTML correspondiente o que entienda el diagrama del servidor.

---

## 💻 3. Gestión del Espacio de Trabajo e Interfaz

*   **Aislamiento de Contexto (Context Isolation):** 
    *   Si abres una nueva pestaña de Wave Terminal (ej. `T2`), Gulin inicia una **conversación completamente en blanco**. Solo leerá el código y la ruta de *esa* pestaña específica. Esto es vital para no mezclar contextos cuando trabajas en el *Frontend* en la pestaña 1, y en el *Backend* en la pestaña 2 de manera concurrente.
*   **Limpiar Memoria Corta (`⌘ K` o `Ctrl + K`):** Si no quieres abrir otra pestaña pero la conversación actual ya divagó mucho sobre un tema, usa este atajo para resetear la memoria a corto plazo del chat e iniciar de cero en el mismo panel de comandos.
*   **Modo Sandbox Seguro:** Si apagas el *Widget Context (OFF)* Gulin entra en modo de "caja de arena". Queda ciego voluntariamente a tu código, rutas y terminales. Pasa a funcionar como una enciclopedia virtual estándar. Usa este modo si necesitas preguntarle a Gulin sobre temas externos (ej. *"¿Cómo escribo un correo para mi jefe?"*) asegurando privacidad absoluta de tu código.

---

## 💡 4. Casos de Uso Diarios Recomendados

Para exprimir todo el potencial de Gulin IA, comunica tus intenciones de manera clara aprovechando que él ya está "viendo" tu entorno:

*   **⚡ Resolución de Errores ("Fix it"):**
    *   *Tú (luego de ver texto rojo en la terminal de build):* *"@Gulin, revisa el error de la terminal. Al parecer falla la compilación de Webpack. Explícame por qué pasó y reescribe el bloque del `webpack.config.js` que causa el fallo."*
*   **⚡ Onboarding en Código Legado:**
    *   *Tú (abriendo un proyecto de hace 3 años):* *"@Gulin, basado en los archivos que ves en este directorio, hazme un diagrama o resumen de cómo fluye la autenticación de usuarios aquí, desde el router hasta el controlador."*
*   **⚡ Refactorización Avanzada y Pruebas:**
    *   *Tú:* *"@Gulin, lee el archivo en pantalla (`auth_service.go`). Refactorízalo para inyectar las dependencias en una interfaz, y luego genera automáticamente en la terminal el comando para ejecutar todos los Unit Tests go de esa carpeta."*
*   **⚡ Guardado de Reglas Activas:**
    *   *Tú:* *"@Gulin, guárdalo en tu Memoria (Brain): A partir de ahora, todo el código CSS de este directorio debe usar la metodología BEM estricta."* 

***
> **Nota de Privacidad y Seguridad:** Gulin está diseñado para ser un *Aumento de la Inteligencia*, no un reemplazo no supervisado. Todas las acciones de borrado de archivos, ejecución de comandos destructivos o envíos a redes externas pausarán el proceso y requerirán tu aprobación en la interfaz (Aceptación de Herramienta) para garantizar el control total sobre tu sistema.
