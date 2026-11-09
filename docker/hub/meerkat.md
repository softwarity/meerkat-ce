# Meerkat

**An authentication proxy and API gateway that puts access control in front of
your internal applications.** It signs people in, decides who may reach which
application and which endpoint, routes the request, and records what happened.
Your services receive requests that are already authenticated, with a signed JWT
carrying the identity, the roles and the tenant.

One binary, no external dependency: no database to install, no cache, no broker.

This image is **every edition in one**: without a license it runs as an
evaluation, with a notice on its pages, and is not for production; a license
makes it Team or Enterprise. The free Community edition is
[`softwarity/meerkat-ce`](https://hub.docker.com/r/softwarity/meerkat-ce).

## Try it

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

- **http://localhost:9090** is the administration console: sign in as `admin`
  with the password you chose.
- **http://localhost:8080** is what your users reach. A fresh gateway has no
  route, so it answers 404 until you add one.

## Tags

- `X.Y.Z`, `X.Y`, `X`: a release, its latest patch, its latest minor.
- `latest`: the latest build of the main branch.

Images are published for `linux/amd64` and `linux/arm64`, and signed.

## Learn more

- Documentation: https://meerkat.softwarity.dev/
- Quick start: https://meerkat.softwarity.dev/docs/start/quick-start/
- Install with Docker, Compose, Swarm or Helm: https://meerkat.softwarity.dev/docs/start/install/
- Editions and pricing: https://meerkat.softwarity.dev/product/editions/
