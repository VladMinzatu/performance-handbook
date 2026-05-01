# Option 1: Set up Linux on Raspberry Pi for VNC access

These are instructions for installing Ubuntu Server on a Raspberry Pi for headlerss access using VNC:
- Dowload [Raspberry Pi Imager](https://www.raspberrypi.com/software/) and install, then run. In the imager, next:
- Pick the device, e.g. Raspberry Pi 5. Then Next
- Pick the OS -> Other General Purpose OS -> Ubuntu -> Ubuntu Server. 
- ... Configure with hostname, user, password and Wi-Fi credentials.

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
