# ogen-gcs-file-upload

To generate password for the application run, where admin is username and password is password. Set these values in the .env file and pass the hashed password into your request as auth.

`htpasswd -nbBC 10 admin password`

# To test

Run `task hurl`

Edit the ./hurl/test.hurl file to change the requests being sent