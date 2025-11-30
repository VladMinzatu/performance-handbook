# Set up Github access

- Create an ssh key and start the agent and add identity

```
ssh-keygen -t ed25519 -C "<email>"

eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519

cat ~/.ssh/id_ed25519.pub

git config --global user.email "<email>"
git config --global user.name "<name>"
```

- Next, add the ssh key to GH: click on profile → settings → manage ssh key →
- Click new or add ssh key, give it a title like “Ubuntu VM” and paste the public key (see above) → authenticator
- Then git clone should work
