CREATE USER keycloak WITH PASSWORD 'keycloak_dev';
CREATE DATABASE keycloak OWNER keycloak;
REVOKE CONNECT ON DATABASE keycloak FROM PUBLIC;

CREATE USER auth_service WITH PASSWORD 'auth_service_dev';
CREATE DATABASE auth_service OWNER auth_service;
REVOKE CONNECT ON DATABASE auth_service FROM PUBLIC;

CREATE USER user_service WITH PASSWORD 'user_service_dev';
CREATE DATABASE user_service OWNER user_service;
REVOKE CONNECT ON DATABASE user_service FROM PUBLIC;

CREATE USER ride_service WITH PASSWORD 'ride_service_dev';
CREATE DATABASE ride_service OWNER ride_service;
REVOKE CONNECT ON DATABASE ride_service FROM PUBLIC;

CREATE USER matching_service WITH PASSWORD 'matching_service_dev';
CREATE DATABASE matching_service OWNER matching_service;
REVOKE CONNECT ON DATABASE matching_service FROM PUBLIC;

CREATE USER payment_service WITH PASSWORD 'payment_service_dev';
CREATE DATABASE payment_service OWNER payment_service;
REVOKE CONNECT ON DATABASE payment_service FROM PUBLIC;