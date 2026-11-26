# A throwaway nOS for the live UI pass: the real nosd and nos-agent with the
# system tools they drive (Samba, NFS, ACLs, Btrfs, SMART, nftables). There
# is no systemd, so service reloads only warn. Built by scripts/live-ui/run.sh.
FROM debian:bookworm

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      samba smbclient acl nfs-kernel-server btrfs-progs smartmontools nftables \
      iproute2 procps util-linux passwd ca-certificates curl tar gzip \
 && rm -rf /var/lib/apt/lists/*

RUN useradd --system --home-dir /var/lib/nos --shell /usr/sbin/nologin nos \
 && mkdir -p /etc/nos /var/lib/nos /srv/shares /srv/apps /etc/samba/smb.conf.d /etc/exports.d \
 && chown -R nos:nos /etc/nos /var/lib/nos \
 && printf '\ninclude = /etc/samba/smb.conf.d/nos-shares.conf\n' >> /etc/samba/smb.conf \
 && touch /etc/samba/smb.conf.d/nos-shares.conf

COPY nosd /usr/bin/nosd
COPY nos-agent /usr/sbin/nos-agent
COPY apps /usr/share/nos/apps
COPY entrypoint.sh /entrypoint.sh

EXPOSE 9000
ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
