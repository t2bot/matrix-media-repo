#!/usr/bin/env sh
su postgres -c "postgres -h 0.0.0.0" &
sleep 1
su postgres -c "psql -c \"create user mediarepo with password 'mediarepo';\""
su postgres -c "psql -c \"create database mediarepo;\""
su postgres -c "psql -c \"grant all privileges on database mediarepo to mediarepo;\""
