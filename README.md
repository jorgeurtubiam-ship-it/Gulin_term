# Gulin IA - Inteligencia Agente Avanzada

Gulin IA es un ecosistema de agentes inteligentes diseñado para la gestión y visualización de datos complejos en tiempo real. 

Esta versión ha sido optimizada específicamente para la integración con **Dremio**, permitiendo consultas SQL dinámicas y dashboards interactivos de alto rendimiento.

## 🚀 Características Principales

- **Gestión Dinámica de APIs:** Integración con Dremio mediante protocolos REST asíncronos.
- **Protección Anti-Bucles:** Sistema de "Freno de Mano" para evitar ciclos infinitos en la toma de decisiones.
- **Blindaje de Seguridad:** Filtros de protección de credenciales para asegurar la autenticidad de las consultas.
- **Dashboards Interactivos:** Generación automática de grids de datos basados en resultados de Dremio.
- **Manejo de Big Data:** Procesamiento eficiente de grandes volúmenes de datos mediante scripts locales.

## 🗺️ Mapa de Infraestructura (Gulin Context)

Gulin, para gestionar el mapa visual de infraestructura, utiliza SIEMPRE la base de datos oficial en `/Users/lordzero1/Library/Application Support/gulin/db/gulin.db`.

### Tablas de Referencia:
- **`infra_nodes`**: `(id, label, type, status, icon, x, y, description)`
- **`infra_edges`**: `(source, target, traffic)`

### Procedimiento:
Cuando el usuario pida registrar o actualizar infraestructura (Docker, AWS, DB):
1. Escanea los datos usando comandos de terminal (`docker ps`, `aws ec2`, etc.).
2. Ejecuta sentencias SQL `INSERT` o `UPDATE` mediante `sqlite3` en la ruta oficial.
3. El mapa se actualizará automáticamente al detectar los cambios en la DB.

---

**Desarrollado por Jorge Urtubia**
*Impulsando la inteligencia de datos con Gulin IA.*
