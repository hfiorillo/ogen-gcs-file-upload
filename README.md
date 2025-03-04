# ogen-gcs-file-upload

To generate password for the application run, where admin is username and password is password.
Set these values in the .env file and pass the hashed password into your request as auth.

`htpasswd -nbBC 10 admin password`

See `.env.example` for an example of this.
`./hurl/test.hurl` contains the hashed password in the request

- Currently limited to:
  - 10Mb upload
  - csv, xlsx file types

# Deploying to GCP Cloud Run

1. Create gcs bucket
2. Add secret manager secret for basic auth username and password
3. Create service account for cloudrun instance
4. Assign service account permissions to access secret from (1) and access to the bucket (read and write)
5. Build image and push to container registry
6. Create cloudrun instance, reference the image from (5). Add required environment variables to cloud run deployment and reference secrets from (2)
```sh
    GcsProject    string `env:"GCS_PROJECT,required,notEmpty"`
    GcsBucketName string `env:"GCS_BUCKET_NAME,required,notEmpty"`
    AuthUsername string `env:"AUTH_USERNAME,required,notEmpty"`
    AuthPassword string `env:"AUTH_PASSWORD,required,notEmpty"`
```
7. Ensure the cloudrun deployment is accessible publicly

# To run

Run `task run`

# To generate ogen

Run `task generate`

# To test

Run `task hurl`

Edit the `./hurl/test.hurl` file to change the requests being sent