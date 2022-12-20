
# Install dependencies, download tor-source, extract, configure, compile tor
(may ask for openssl libraries as well)
```bash
sudo torsocks apt install -y gcc make libsystemd-dev checkinstall 
# sudo torsocks apt install -y gcc libsystemd-dev checkinstall lttng-modules-dkms liblttng-ust-dev lttng-tools systemtap-sdt-dev
prefix_working="/tmp/tortest"
prefix_install="tmp/tortest"
tor_version="0.4.7.3-alpha"
mkdir -p "${prefix_working}"
pushd "${prefix_working}"
torsocks wget -U "Mozilla/5.0 (Windows NT 10.0; rv:91.0) Gecko/20100101 Firefox/91.0" "https://dist.torproject.org/tor-${tor_version}.tar.gz"
tar zxvf "tor-${tor_version}.tar.gz" --directory "${prefix_working}"
mv "${prefix_working}/tor-${tor_version}/" "${prefix_working}/tor-source"
pushd tor-source
./configure --prefix="${prefix_install}" --enable-systemd \
--disable-asciidoc --disable-manpage --disable-html-manual
#--enable-tracing-instrumentation-lttng --enable-tracing-instrumentation-usdt --enable-tracing-instrumentation-log-debug \
make
popd
```



# Where's it all going
```info
Install Directories
  Binaries:                                                      /tmp/tortest/bin
  Configuration:                                                 /tmp/tortest/etc/tor
  Man Pages:                                                     /tmp/tortest/share/man

```

# Package .deb and install tor-from-source
```bash
tor_version="0.4.7.3-alpha"
prefix_working="/tmp/tortest"
pushd "${prefix_working}/tor-source"
mkdir doc-pak
printf "tor: compiled from source.
- with instrumentation options: liblttng, systemtap, log-debug
" | tee description-pak
cp README INSTALL ChangeLog LICENSE description-pak doc-pak
popd

alias pack='pushd "${prefix_working}/tor-source";sudo checkinstall -y --install=no --maintainer=local@local --pkgversion="${tor_version}" --pkgname=tor-from-source --provides=tor-from-source;popd'
alias inst='pushd "${prefix_working}/tor-source";sudo dpkg -i "tor-from-source_${tor_version}-1_amd64.deb";popd'
alias remo='pushd "${prefix_working}/tor-source";sudo dpkg -r tor-from-source_0.4.7.3-alpha-1_amd64.deb;popd'
pack
inst
```