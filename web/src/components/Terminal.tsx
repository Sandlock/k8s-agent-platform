import { useEffect, useRef } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { terminalWsUrl } from '../api'
import '@xterm/xterm/css/xterm.css'

interface Props { sandboxId: string }

export default function Terminal({ sandboxId }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    const term = new XTerm({ theme: { background: '#0d1117' }, cursorBlink: true, fontSize: 14 })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current!)
    fit.fit()
    termRef.current = term

    const ws = new WebSocket(terminalWsUrl(sandboxId))
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ rows: term.rows, cols: term.cols }))
    }
    ws.onmessage = (e) => {
      const data = e.data instanceof ArrayBuffer ? new Uint8Array(e.data) : e.data
      term.write(data)
    }
    ws.onclose = () => term.writeln('\r\n\x1b[31m[disconnected]\x1b[0m')

    const encoder = new TextEncoder()
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(data))
    })

    term.onResize(({ rows, cols }) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ rows, cols }))
    })

    const observer = new ResizeObserver(() => fit.fit())
    observer.observe(containerRef.current!)

    return () => {
      ws.close()
      term.dispose()
      observer.disconnect()
    }
  }, [sandboxId])

  return <div ref={containerRef} style={{ flex: 1, background: '#0d1117' }} />
}
