# 🚀 Instalador Local de GuLiN Agent para macOS

## 📁 Archivos del instalador

| Archivo | Descripción |
|---------|-------------|
| `install_gulin.sh` | **Instalación completa**: verifica deps, compila backend + frontend, empaqueta .app y lo instala en `/Applications` |
| `install_quick.sh` | **Instalación rápida** (desarrollo): solo compila backend, usa `npm run dev` con hot-reload |
| `uninstall_gulin.sh` | Desinstalador que elimina la app, PATH, cachés y config |

## ⚡ Instalación completa (producción)

```bash
cd /Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm
chmod +x install_gulin.sh
./install_gulin.sh
```

Esto hará:
1. ✅ Verificar Go, Node.js, Task, Zig y Xcode CLI Tools
2. ✅ Verificar estructura del proyecto
3. ✅ `npm install` (dependencias frontend)
4. ✅ `go mod download` (dependencias backend)
5. ✅ Compilar `gulinsrv` y `wsh` (backend Go)
6. ✅ `npm run build:prod` (frontend Electron)
7. ✅ Empaquetar `.app` con electron-builder
8. ✅ Instalar en `/Applications/GuLiN.app`
9. ✅ Configurar CLI (`wsh`, `gulinsrv`) en PATH
10. ✅ Opción de fijar al Dock

**Tiempo estimado:** 15-20 minutos (depende de velocidad de descarga y compilación)

## 🔧 Instalación rápida (desarrollo)

```bash
cd /Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm
chmod +x install_quick.sh
./install_quick.sh
```

Esto compila solo el backend e inicia el servidor de desarrollo Vite con hot-reload.

## 🗑️ Desinstalar

```bash
cd /Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm
chmod +x uninstall_gulin.sh
./uninstall_gulin.sh
```

## 📋 Prerequisitos

Asegúrate de tener instalado:

```bash
# Verificar versión de macOS (Sequoia 15.4+)
sw_vers

# Instalar lo necesario vía Homebrew
brew install go          # Go 1.25+
brew install node        # Node.js 22 LTS
brew install go-task     # Task runner
brew install zig         # Zig (CGO estático, opcional)

# Xcode Command Line Tools
xcode-select --install
```
