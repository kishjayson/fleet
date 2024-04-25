FROM fleetdm/fleet:v4.48.3
CMD fleet prepare --no-prompt=true db && fleet serve
