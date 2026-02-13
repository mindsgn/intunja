# Intunja - Quick Reference Guide

## 🚀 Installation

```bash
git clone https://github.com/yourusername/intunja
cd intunja
go build -o intunja
./intunja
```

---

## ⌨️ Keyboard Shortcuts

### Main View

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up in list |
| `↓` / `j` | Move down in list |
| `Enter` | View details |
| `a` | Add torrent file |
| `m` | Add magnet link |
| `s` | Start torrent |
| `p` | Pause torrent |
| `d` | Delete torrent |
| `c` | View config |
| `q` | Quit |

### Details View

| Key | Action |
|-----|--------|
| `Esc` | Back |
| `s` | Start |
| `p` | Pause |
| `d` | Delete |

### Input Mode

| Key | Action |
|-----|--------|
| `Enter` | Submit |
| `Esc` | Cancel |
| `Backspace` | Delete char |

---

## 📊 Status Indicators

| Status | Meaning |
|--------|---------|
| **Loading...** | Fetching metadata |
| **Active** | Downloading |
| **Seeding** | Complete, uploading |
| **Stopped** | Paused |

---

## 🔧 Configuration

### Default Settings

```json
{
  "AutoStart": true,
  "DownloadDirectory": "./downloads",
  "EnableUpload": true,
  "EnableSeeding": true,
  "IncomingPort": 50007
}
```

### Change Port

```bash
# Edit config or pass flag
./intunja --port 6881
```

---

## 🐛 Common Issues

### No Peers
- Check tracker status
- Verify port forwarding
- Enable DHT

### Slow Download
- Enable upload
- Check internet speed
- Choose well-seeded torrents

### Port in Use
- Change port in config
- Kill other torrent clients

---

## 📁 File Locations

| Item | Location |
|------|----------|
| Downloads | `./downloads` |
| Config | `config.json` |
| Executable | `./intunja` |

---

## 🔗 Magnet Link Format

```
magnet:?xt=urn:btih:HASH&dn=NAME&tr=TRACKER
```

**Components**:
- `xt`: eXact Topic (info hash)
- `dn`: Display Name
- `tr`: Tracker URL

---

## 💡 Pro Tips

1. **Port Forward** `50007` for more peers
2. **Keep Encryption** enabled for privacy
3. **Enable Upload** for better speeds
4. **Seed After Download** to help the swarm
5. **Use SSH** for remote management

---

## 📈 Performance

| Metric | Value |
|--------|-------|
| Memory | 15-80 MB |
| CPU (idle) | <1% |
| CPU (active) | 5-15% |
| Startup time | <200ms |

---

## 🌐 Protocol Info

- **Default Port**: 50007
- **Protocol**: BitTorrent
- **Encryption**: Enabled
- **DHT**: Supported
- **PEX**: Supported

---

## 📞 Support

- Issues: [GitHub](https://github.com/yourusername/intunja/issues)
- Docs: [README](README.md)
- Email: support@example.com

---

## 🔐 Security

- ✅ Protocol encryption
- ✅ Local-only (no remote access)
- ✅ No data collection
- ❌ No VPN built-in (use external VPN)

---

**Version**: 2.0.0 | **License**: MIT