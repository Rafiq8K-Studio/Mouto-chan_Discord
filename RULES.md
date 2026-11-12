1. hal yang berkaitan dengan data sensitif wajib menggunakan .env dan .env.example
2. bisa menggunakan docker dan air go
3. Modular, jadi misal ada modules/age/*, atau modules/ping.go
4. perlu diingat jika ada yang perlu diluar modules, maka itu harus bersifat universal, tidak boleh hanya untuk satu module itu saja, harus bisa re-use untuk module-module lainnya