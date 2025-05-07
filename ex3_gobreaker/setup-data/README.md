# PostgreSQL Docker Setup

This project sets up a PostgreSQL database using Docker. It includes the necessary configuration files to run the database and initialize it with a predefined schema.

## Project Structure

```
postgres-docker-setup
├── docker-compose.yml
├── .env
├── init-scripts
│   └── init-db.sql
└── README.md
```

## Requirements

- Docker
- Docker Compose

## Setup Instructions

1. **Clone the Repository**

   Clone this repository to your local machine:

   ```
   git clone <repository-url>
   cd postgres-docker-setup
   ```

2. **Configure Environment Variables**

   Update the `.env` file with your desired database configuration. The file should contain the following variables:

   ```
   POSTGRES_DB=your_database_name
   POSTGRES_USER=your_username
   POSTGRES_PASSWORD=your_password
   ```

3. **Start the Docker Containers**

   Use Docker Compose to start the PostgreSQL container:

   ```
   docker-compose up -d
   ```

   This command will start the PostgreSQL service in detached mode.

4. **Initialize the Database**

   The `init-db.sql` script located in the `init-scripts` directory will be executed automatically when the PostgreSQL container starts. This script sets up the database schema as defined in the provided SQL code.

5. **Accessing the Database**

   You can connect to the PostgreSQL database using any PostgreSQL client with the following credentials:

   - Host: `localhost`
   - Port: `5432`
   - Database: specified in `.env`
   - User: specified in `.env`
   - Password: specified in `.env`

## Stopping the Containers

To stop the running containers, use:

```
docker-compose down
```

## Additional Information

For more details on Docker and PostgreSQL, refer to the official documentation:

- [Docker Documentation](https://docs.docker.com/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)