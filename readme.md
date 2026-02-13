# Intunja - BitTorrent CLI Client

A powerful, terminal-based BitTorrent client built with Go and Bubble Tea. Originally a web-based application, now refactored into a beautiful CLI experience.

![Version](https://img.shields.io/badge/version-2.0.0-blue.svg)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

---

## 🌟 Features

### Core Functionality
- ✅ **Add torrents** via .torrent files or magnet links
- ✅ **Real-time progress tracking** with live download statistics
- ✅ **Start/pause/delete** individual torrents
- ✅ **Automatic seeding** after download completion
- ✅ **Multi-file torrent support** with per-file progress
- ✅ **Configuration management** with persistent settings

### Terminal UI
- 🎨 **Beautiful TUI** powered by Bubble Tea
- 📊 **Live statistics** - download rates, progress, peer counts
- ⌨️ **Keyboard navigation** - vi-style bindings
- 📱 **Responsive layout** - adapts to terminal size
- 🎯 **Multiple views** - main list, details, settings

### Advanced Features
- 🚀 **High performance** - built on anacrolix/torrent library
- 🔒 **Protocol encryption** - hide BitTorrent traffic from ISPs
- 📁 **Automatic directory creation** - organized downloads
- 💾 **Persistent state** - resume downloads after restart
- 🌐 **DHT support** - trackerless torrent discovery

---

## 📋 Requirements

- **Go 1.21 or later**
- **Terminal with 256-color support** (most modern terminals)
- **Minimum terminal size**: 80x24 characters

---

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/intunja
cd intunja

# Install dependencies
go mod download

# Build
go build -o intunja

# Run
./intunja
```

### First Launch

1. The application will create a `downloads` directory in the current folder
2. Default port: `50007` (can be changed in settings)
3. Press `a` to add your first torrent

---

## 📖 Usage Guide

### Main Screen

The main screen shows all your torrents in a table format:

```
┌─────────────────────────────────────────────────────────────────┐
│ Name                      │ Progress │ Size   │ Down    │ Status │
├─────────────────────────────────────────────────────────────────┤
│ ubuntu-22.04-desktop.iso  │ 45.2%    │ 3.2 GB │ 5.1 MB/s│ Active │
│ my-archive.zip            │ 100.0%   │ 1.5 GB │ 0 B/s   │ Seeding│
│ large-dataset.tar.gz      │ 12.8%    │ 8.9 GB │ 2.3 MB/s│ Active │
└─────────────────────────────────────────────────────────────────┘
```

### Keyboard Shortcuts

#### Main View
| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate torrent list |
| `Enter` | View torrent details |
| `a` | Add torrent from file |
| `m` | Add torrent from magnet link |
| `s` | Start selected torrent |
| `p` | Pause selected torrent |
| `d` | Delete selected torrent |
| `c` | View configuration |
| `q` | Quit application |

#### Details View
| Key | Action |
|-----|--------|
| `Esc` | Back to main view |
| `s` | Start this torrent |
| `p` | Pause this torrent |
| `d` | Delete this torrent |

#### Input Mode (Adding Torrents)
| Key | Action |
|-----|--------|
| `Enter` | Submit input |
| `Esc` | Cancel |
| `Backspace` | Delete character |

---

## 🔧 Configuration

### Default Configuration

```json
{
  "AutoStart": true,
  "DisableEncryption": false,
  "DownloadDirectory": "./downloads",
  "EnableUpload": true,
  "EnableSeeding": true,
  "IncomingPort": 50007
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `AutoStart` | bool | `true` | Start downloading immediately when torrent is added |
| `DisableEncryption` | bool | `false` | Disable protocol encryption (not recommended) |
| `DownloadDirectory` | string | `"./downloads"` | Directory where files are saved |
| `EnableUpload` | bool | `true` | Allow uploading to other peers |
| `EnableSeeding` | bool | `true` | Continue uploading after download completes |
| `IncomingPort` | int | `50007` | Port for incoming peer connections |

### Changing Configuration

Configuration can be modified in two ways:

1. **Via CLI** (planned feature):
   ```bash
   ./intunja --port 6881 --download-dir ~/Downloads
   ```

2. **Via config file**:
   ```bash
   ./intunja --config /path/to/config.json
   ```

---

## 🏗️ Architecture

### Project Structure

```
intunja/
├── cmd/
│   └── cli.go           # CLI implementation with Bubble Tea
├── engine/
│   ├── config.go        # Configuration structure
│   ├── engine.go        # Engine core (torrent management)
│   └── torrent.go       # Torrent state tracking
├── main.go              # Application entry point
├── go.mod               # Go module definition
└── README.md
```

### Component Diagram

```
┌────────────────────────────────────────┐
│         Terminal UI (Bubble Tea)        │
│  ┌──────────┐  ┌──────────┐  ┌───────┐│
│  │Main View │  │ Details  │  │Config ││
│  └──────────┘  └──────────┘  └───────┘│
└────────────────┬───────────────────────┘
                 │
                 ↓
┌────────────────────────────────────────┐
│          Engine (State Manager)         │
│  ┌─────────────────────────────────┐  │
│  │  Torrent Map (InfoHash → State) │  │
│  └─────────────────────────────────┘  │
└────────────────┬───────────────────────┘
                 │
                 ↓
┌────────────────────────────────────────┐
│      anacrolix/torrent Library         │
│  (BitTorrent Protocol Implementation)  │
└────────────────────────────────────────┘
```

### Key Components

1. **CLI Layer** (`cmd/cli.go`)
   - Handles user input
   - Renders UI with Bubble Tea
   - Updates every second (ticker)
   - Manages view state

2. **Engine Layer** (`engine/`)
   - Wraps anacrolix/torrent library
   - Maintains torrent state
   - Handles configuration
   - Thread-safe operations

3. **Storage Layer** (anacrolix/torrent)
   - Piece verification
   - Disk I/O
   - Peer management
   - DHT/tracker communication

---

## 🎯 Adding Torrents

### From Magnet Link

1. Press `m` in the main view
2. Paste your magnet link:
   ```
   magnet:?xt=urn:btih:HASH&dn=NAME&tr=TRACKER
   ```
3. Press `Enter`

The client will:
- Connect to DHT/trackers
- Download torrent metadata
- Start downloading automatically (if `AutoStart` is enabled)

### From .torrent File

1. Press `a` in the main view
2. Enter the path to your .torrent file:
   ```
   /path/to/file.torrent
   ```
3. Press `Enter`

**Note**: Relative and absolute paths are supported.

---

## 📊 Understanding the Display

### Progress Bar

```
█████████░░░░░░░░░░ 45.2%
```

- **Filled portion** (█): Downloaded
- **Empty portion** (░): Remaining
- **Percentage**: Overall completion

### Download Rate

Shows current download speed:
- `5.1 MB/s` - Actively downloading
- `0 B/s` - Paused or complete

### Status

- **Loading...**: Fetching metadata
- **Active**: Downloading pieces
- **Seeding**: Complete, uploading to others
- **Stopped**: Paused by user

---

## 🐛 Troubleshooting

### No peers found

**Symptoms**: Torrent stuck at 0%, status shows "Active" but no download

**Solutions**:
1. Check if torrent is still actively seeded
2. Try adding more trackers (via magnet link)
3. Ensure your firewall allows incoming connections on the configured port
4. Enable DHT (default: enabled)

### Port already in use

**Symptoms**: Error on startup: "Invalid incoming port"

**Solutions**:
1. Change the port in configuration
2. Check if another BitTorrent client is running
3. Use a port in the range 49152-65535 (dynamic ports)

### Downloads not starting

**Symptoms**: Torrents added but never start downloading

**Solutions**:
1. Check `AutoStart` setting (press `c` to view config)
2. Manually start torrent by pressing `s`
3. Verify `DownloadDirectory` is writable

### Slow downloads

**Symptoms**: Download speed much slower than expected

**Solutions**:
1. Check if `EnableUpload` is disabled (reduces peer connections)
2. Verify your internet connection speed
3. Try torrents with more seeders
4. Check firewall/router settings for port forwarding

---

## 🔒 Security & Privacy

### Protocol Encryption

By default, the client uses **protocol encryption** to hide BitTorrent traffic from ISPs:

```json
{
  "DisableEncryption": false  // Keep this false for privacy
}
```

**How it works**:
- Encrypts peer wire protocol messages
- Makes traffic look like random data
- Prevents ISP throttling based on protocol detection

**Note**: This is NOT end-to-end encryption. It only obfuscates the protocol.

### Port Forwarding

For optimal connectivity, configure port forwarding on your router:

1. Find your local IP: `ifconfig` (Linux/Mac) or `ipconfig` (Windows)
2. Access router admin panel (usually `192.168.1.1`)
3. Forward port `50007` (or your configured port) to your local IP
4. Protocol: TCP

**Why?**: Allows peers to initiate connections to you, increasing swarm participation.

---

## 🚧 Known Limitations

1. **No pause/resume**: Stopping a torrent requires re-adding to resume (anacrolix limitation)
2. **No per-file control**: Cannot start/stop individual files in multi-file torrents
3. **No bandwidth limiting**: Global upload/download speed caps not yet implemented
4. **No search**: Built-in torrent search removed in CLI version
5. **No watch folder**: Cannot auto-add torrents from a directory

---

## 🗺️ Roadmap

### Version 2.1
- [ ] Bandwidth throttling (max upload/download speed)
- [ ] Per-file priority selection
- [ ] Watch folder for auto-adding torrents
- [ ] Import/export torrent list

### Version 2.2
- [ ] RSS feed monitoring
- [ ] Scheduled downloads (start/stop at specific times)
- [ ] Remote control via REST API
- [ ] Plugin system for custom scrapers

### Version 3.0
- [ ] Web UI (bring back web interface as optional)
- [ ] Mobile app (Android/iOS)
- [ ] Cloud sync of torrent list
- [ ] Advanced statistics and graphs

---

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/intunja
cd intunja

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o intunja

# Run
./intunja
```

---

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

---

## 🙏 Acknowledgments

- **anacrolix/torrent** - Excellent BitTorrent library
- **Bubble Tea** - Beautiful TUI framework
- **Lipgloss** - Terminal styling
- **Original cloud-torrent project** - Inspiration for the engine design

---

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/yourusername/intunja/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/intunja/discussions)
- **Email**: support@example.com

---

## 📈 Performance Tips

### Maximizing Download Speed

1. **Enable port forwarding** on your router
2. **Keep upload enabled** - improves peer reciprocity
3. **Choose well-seeded torrents** - more seeders = faster downloads
4. **Avoid ISP throttling** - keep encryption enabled
5. **Close other bandwidth-heavy applications**

### Minimizing Resource Usage

1. **Limit active torrents** - pause unused downloads
2. **Reduce seeding after completion** - set `EnableSeeding: false`
3. **Use smaller piece sizes** - reduces memory usage (can't change per-torrent)

---

## 🔍 FAQ

**Q: Can I run this on a server without a display?**  
A: Yes! The TUI works over SSH and doesn't require a graphical environment.

**Q: Does this work on Windows?**  
A: Yes, but Windows Terminal or WSL is recommended for best experience.

**Q: Can I use this with a VPN?**  
A: Yes, but ensure your VPN supports port forwarding for optimal performance.

**Q: How do I resume a stopped torrent?**  
A: Select the torrent and press `s`. If it was deleted, you'll need to re-add it.

**Q: Where are the downloaded files?**  
A: By default in `./downloads` relative to where you run the binary. Check with `c` (config view).

**Q: Can I import torrents from another client?**  
A: Not directly. You'll need to re-add them via magnet links or .torrent files.

---

**Made with ❤️ by the Intunja team**