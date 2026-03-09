---
sidebar_position: 2
title: Quick Start
---

# Quick Start

Get Togglerino running locally in under five minutes using Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed on your machine

## Step 1: Create a Docker Compose File

Create a `docker-compose.yml` file in an empty directory:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: togglerino
      POSTGRES_PASSWORD: togglerino
      POSTGRES_DB: togglerino
    volumes:
      - pgdata:/var/lib/postgresql/data

  togglerino:
    image: ghcr.io/togglerino/togglerino:latest
    ports:
      - "8090:8080"
    environment:
      DATABASE_URL: postgres://togglerino:togglerino@postgres:5432/togglerino?sslmode=disable
      PORT: "8080"
      CORS_ORIGINS: "*"
      LOG_FORMAT: json
    depends_on:
      - postgres

volumes:
  pgdata:
```

:::tip
If you want to build from source instead, clone the repository and replace `image: ghcr.io/togglerino/togglerino:latest` with `build: .`.
:::

## Step 2: Start the Services

Run Docker Compose to start PostgreSQL and Togglerino:

```bash
docker compose up
```

Wait until you see the server startup log message. Database migrations run automatically on first start.

## Step 3: Create Your Admin Account

Open **[http://localhost:8090](http://localhost:8090)** in your browser.

:::note
Docker Compose maps host port **8090** to the container's port 8080. If you run the binary directly outside of Docker, the dashboard is at `http://localhost:8080` instead.
:::

On first launch, Togglerino shows a setup screen. Enter an email address and password to create your admin account. This is the only time the setup screen appears — once an admin exists, it switches to the normal login page.

## Step 4: Create Your First Project

After logging in, click **Create Project** and give it a name (for example, "My App") and a key (for example, `my-app`).

Projects are the top-level grouping for your flags. When you create a project, Togglerino automatically creates three environments: **development**, **staging**, and **production**. Each environment has independent flag configurations, so you can enable a flag in development without affecting production.

## Step 5: Create a Boolean Flag

1. Navigate to your new project and click **Create Flag**
2. Enter a name like "New Checkout Flow" and a key like `new-checkout-flow`
3. Choose **Boolean** as the value type and **Release** as the flag type
4. After creating the flag, go to its detail page and select the **development** environment
5. Toggle the flag to **enabled** and set the default variant to `true`
6. Save the configuration

Your flag is now live in the development environment. Any SDK connecting with a development SDK key will receive this flag's value.

## Next Steps

Your Togglerino instance is running and you have a flag configured. Next, wire it up to your application code using one of the client SDKs.

Continue to [First Flag in Code](./first-flag-in-code.md) to connect an SDK and evaluate your flag.
