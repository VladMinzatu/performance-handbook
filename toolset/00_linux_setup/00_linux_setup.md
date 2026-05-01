# Option 1: Set up Linux on Raspberry Pi in headless mode

These are instructions for installing Ubuntu Server on a Raspberry Pi for headless access:
- Dowload [Raspberry Pi Imager](https://www.raspberrypi.com/software/) and install.
- Now run Raspberry Pi Imager -> Pick device (e.g. Raspberry Pi 5) -> For the OS, pick `Other General-purpose OS` -> pick `Ubuntu Server` and configure with hostname, username, password and wifi credentials and enable ssh. Then Write to the SD card.
- When done, plug SD card in the raspberry pi and start. No need to connect to peripherals, we can run fully headless, just wait a couple min if necessary.
- Then it should be possible to run `ssh <user>@<hostname>.local` (for user and hostname selected earlier)
  - If this still doesn't eventually work, connect monitor and keyboard+mouse to raspberry pi and debug (most likely wifi connection didn't work and can be checked via `ip a` and configuration fixed by editing `/etc/netplan/*.yaml` and running `sudo netplan apply`) 


# Option 2: Set Up Linux VM with UTM (on Mac)

These are instructions for creating a new VM using UTM on Mac with a fresh installation of Ubuntu server.

- Ubuntu Desktop won’t work on UTM, you need to download Ubuntu server and then install Ubuntu Desktop (https://docs.getutm.app/guides/ubuntu/)
- Go to https://ubuntu.com/download/server/arm to download Ubuntu server for ARM.
- Open UTM and click Create New VM
- Choose **Virtualize** and match the VM (ARM VM) - Emulate would be slower. Then choose Linux.
- On Boot, click Browse and select the server ISO file and continue.
- Then specify memory (preferably ≥8GB) and cores assign 4 if your Mac has 8+ cores. And for storage 64 GB.
- Save and then Run the VM.
- Go through the Ubuntu installer. At the end, you’ll have the option to “Reboot Now,” but after selecting that option and rebooting, the reboot may fail. (It will hang at a black screen with a blinking cursor.) This is expected!
- If the VM did not restart automatically, manually quit the VM, unmount the installer ISO (Edit, USB, clear removable), and start the VM again to boot into your new installation.

- After restart, install Ubuntu desktop for the UI

```
$ sudo apt update
$ sudo apt install ubuntu-desktop
$ sudo reboot
```
