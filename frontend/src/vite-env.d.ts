/// <reference types="vite/client" />

/* eslint-disable @typescript-eslint/no-explicit-any */
declare module 'zmodem.js' {
  const Zmodem: any
  export default Zmodem
  export const Sentry: any
  export const Browser: any
  export const ZMLIB: any
  export const Session: any
}
declare module 'zmodem.js/src/zmodem_browser.js' {
  const Zmodem: any
  export default Zmodem
}
/* eslint-enable @typescript-eslint/no-explicit-any */
