# pkgtui

Un'interfaccia a terminale (TUI) in stile **htop** per cercare, installare,
rimuovere e aggiornare pacchetti **apt** e **snap**, da un'unica dashboard,
senza dover ricordare la sintassi dei due gestori di pacchetti.

```
  pkgtui                                                              apt      snap
 APT — Installati (543)
● bash                         5.1-6ubuntu1.1
● curl                         7.81.0-1ubuntu1.25
▲ nftables                     1.0.2-1ubuntu3
● python3                      3.10.6-1~22.04.1
...
  → apt/snap  tab vista  / cerca  enter dettagli  i installa  d rimuovi  u aggiorna  U aggiorna tutto  s sync  q esci
```

## Funzionalità

- **Due backend distinti**: tab separate per `apt` e `snap`, ciascuna con il
  proprio stato (i due mondi non vengono mischiati).
- **Ricerca live**: cerca pacchetti per nome/descrizione (`apt-cache search`
  / `snap find`).
- **Installati / Aggiornabili**: viste dedicate per vedere subito cosa hai
  installato e cosa ha un aggiornamento disponibile.
- **Dettagli pacchetto**: descrizione, versione, dipendenze (apt) o
  publisher/canali (snap) prima di installare.
- **Installazione/rimozione/aggiornamento con conferma**: ogni azione
  privilegiata chiede conferma (`y`/`n`) prima di essere eseguita.
- **Output live**: i comandi `apt-get`/`snap` vengono eseguiti collegati al
  terminale reale (incluso il prompt della password `sudo`), così vedi
  esattamente cosa succede, come da riga di comando.
- **Aggiorna tutto**: un tasto per lanciare `apt-get upgrade` o `snap
  refresh` su tutti i pacchetti del backend attivo.

## Installazione

Nessuna verifica/store richiesta: scarichi l'asset dalla [pagina delle
release](https://github.com/padovanl/pkgtui/releases) e lo installi in
locale.

### Pacchetto `.deb` (Debian/Ubuntu e derivate)

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<versione>_amd64.deb
sudo apt install ./pkgtui_<versione>_amd64.deb
```

### Pacchetto `.snap` (side-load, senza Snap Store)

`pkgtui` deve poter invocare `apt`/`snap` sul sistema host, quindi il pacchetto
usa confinement `classic`:

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<versione>_amd64.snap
sudo snap install --dangerous --classic pkgtui_<versione>_amd64.snap
```

### Binario standalone (qualsiasi distro Linux con apt e/o snap)

```bash
curl -LO https://github.com/padovanl/pkgtui/releases/latest/download/pkgtui_<versione>_linux_amd64.tar.gz
tar -xzf pkgtui_<versione>_linux_amd64.tar.gz
sudo mv pkgtui /usr/local/bin/
```

### Da sorgente

```bash
git clone https://github.com/padovanl/pkgtui.git
cd pkgtui
go build -o pkgtui .
sudo mv pkgtui /usr/local/bin/
```

## Utilizzo

```bash
pkgtui
```

| Tasto       | Azione                                   |
| ----------- | ----------------------------------------- |
| `←` / `→`   | Cambia backend (apt / snap)               |
| `tab`       | Cambia vista (Installati / Aggiornabili / Ricerca) |
| `/`         | Cerca (poi `invio` per lanciare la ricerca) |
| `↑`/`↓`, `j`/`k` | Naviga la lista                      |
| `enter`     | Mostra i dettagli del pacchetto selezionato |
| `i`         | Installa il pacchetto selezionato          |
| `d`         | Rimuove il pacchetto selezionato           |
| `u`         | Aggiorna il pacchetto selezionato          |
| `U`         | Aggiorna **tutti** i pacchetti del backend attivo |
| `s`         | Sincronizza la cache (`apt-get update`; no-op su snap) |
| `y` / `n`   | Conferma / annulla un'azione               |
| `esc`       | Torna indietro                             |
| `q`         | Esci                                       |

Le azioni che modificano il sistema (installazione, rimozione,
aggiornamento) vengono eseguite con `sudo`: pkgtui cede il terminale al
comando reale, quindi vedrai l'eventuale richiesta della password e l'output
di `apt`/`snap` esattamente come da riga di comando.

## Requisiti

- Linux con `apt`/`dpkg` e/o `snapd` installati (basta averne anche solo uno
  dei due: la tab dell'altro segnala semplicemente "non disponibile").
- `sudo` configurato per l'utente corrente, per le operazioni privilegiate.

## Build & pacchettizzazione (per contribuire)

Il progetto usa [goreleaser](https://goreleaser.com) per compilare i
binari multi-arch e generare il `.deb` (via nfpm integrato), e
[snapcraft](https://snapcraft.io/docs/snapcraft-overview) per il `.snap`
(vedi `snap/snapcraft.yaml`).

```bash
# Build + .deb locali, senza pubblicare nulla
goreleaser release --snapshot --clean --skip=publish

# Pacchetto snap locale
snapcraft pack
```

Le release ufficiali vengono generate automaticamente da GitHub Actions ad
ogni tag `vX.Y.Z` (vedi `.github/workflows/release.yml`).

## Licenza

MIT — vedi [LICENSE](LICENSE).
