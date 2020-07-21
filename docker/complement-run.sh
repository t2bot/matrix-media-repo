#!/usr/bin/env sh
su postgres -c "postgres -h 0.0.0.0" &
sleep 3
/usr/local/bin/media_repo &
/usr/local/bin/complement_hs