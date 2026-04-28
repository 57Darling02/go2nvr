declare module '@/lib/video-rtc.js' {
  export class VideoRTC extends HTMLElement {
    src: string
    mode: string
    media: string
    background: boolean
    visibilityCheck: boolean
    visibilityThreshold: number

    play(): void
    send(value: any): void
    codecs(isSupported: (type: string) => boolean): string

    oninit(): void
    onconnect(): boolean
    ondisconnect(): void
    onopen(): string[]
    onclose(): boolean
    onmse(): void
    onwebrtc(): void
    onmjpeg(): void
    onhls(): void
    onmp4(): void

    static btoa(buffer: ArrayBuffer): string
  }
}
