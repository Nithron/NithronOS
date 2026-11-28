# A throwaway nOS for the live UI pass: the real nosd and nos-agent with the
# system tools they drive (Samba, NFS, ACLs, Btrfs, SMART, nftables) and a
# Docker engine of its own for apps. There is no systemd: the systemctl here
# runs the nos-app@ units only, so other service reloads only warn. Built by
# scripts/live-ui/run.sh.
FROM debian:bookworm

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      samba smbclient acl nfs-kernel-server btrfs-progs smartmontools nftables \
      iproute2 procps util-linux passwd ca-certificates curl tar gzip \
 && rm -rf /var/lib/apt/lists/*

# Docker from Docker's repository, as nos-apps-runtime sets it up on nOS,
# with the other tools that package depends on.
RUN install -d -m 0755 /etc/apt/keyrings \
 && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
 && echo "deb [signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian bookworm stable" > /etc/apt/sources.list.d/docker.list \
 && apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      docker-ce docker-ce-cli containerd.io docker-compose-plugin iptables rsync jq \
 && rm -rf /var/lib/apt/lists/*

RUN useradd --system --home-dir /var/lib/nos --shell /usr/sbin/nologin --groups docker nos \
 && mkdir -p /etc/nos /var/lib/nos /srv/shares /srv/apps /etc/samba/smb.conf.d /etc/exports.d \
 && chown -R nos:nos /etc/nos /var/lib/nos \
 && printf '\ninclude = /etc/samba/smb.conf.d/nos-shares.conf\n' >> /etc/samba/smb.conf \
 && touch /etc/samba/smb.conf.d/nos-shares.conf

COPY nosd /usr/bin/nosd
COPY nos-agent /usr/sbin/nos-agent
COPY apps /usr/share/nos/apps
COPY nos-app-helper.sh nos-app-snapshot.sh /usr/lib/nos/apps/
COPY systemctl /usr/local/bin/systemctl
RUN chmod 0755 /usr/lib/nos/apps/*.sh /usr/local/bin/systemctl
COPY entrypoint.sh /entrypoint.sh

EXPOSE 9000
ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
