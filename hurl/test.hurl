POST http://localhost:8080/upload
[BasicAuth]
admin: password
[MultipartFormData]
field2: file,test.txt;
field3: file,example.zip; application/zip