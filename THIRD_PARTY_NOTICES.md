# Third-party notices

## go2rtc

`internal/hapc/pairverify.go` reimplements the client side of HAP pair-verify following `Client.Dial` in go2rtc v1.9.14, `pkg/hap/client.go` (https://github.com/AlexxIT/go2rtc). go2rtc is also used as a Go module dependency.

```
MIT License

Copyright (c) 2022 Alexey Khit

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## HAP-NodeJS

`internal/catalog/tables.go` is generated from HAP-NodeJS v2.2.3 (https://github.com/homebridge/HAP-NodeJS), files `src/lib/definitions/ServiceDefinitions.ts`, `src/lib/definitions/CharacteristicDefinitions.ts` and `src/lib/HAPServer.ts`.

- Changes: the type names are extracted into Go tables, and the status labels are translated into French.
- License: Apache License 2.0 (https://www.apache.org/licenses/LICENSE-2.0).
- HAP-NodeJS v2.2.3 ships no NOTICE file.
