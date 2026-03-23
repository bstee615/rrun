Name:           rrun
Version:        0.1.0
Release:        1%{?dist}
Summary:        Sync git-tracked files to a remote machine and run commands there

License:        MIT
URL:            https://github.com/bstee615/rrun
Source0:        https://github.com/bstee615/rrun/archive/v%{version}.tar.gz

BuildRequires:  golang >= 1.22

Requires:       openssh
Requires:       rsync
Requires:       git

%description
rrun syncs files tracked by git to a remote machine and runs commands
there with live-streamed output. Zero project buy-in — works with any
git repo, no config files added to your projects.

%prep
%autosetup -n rrun-%{version}

%build
export CGO_ENABLED=0
go build \
    -trimpath \
    -buildvcs=false \
    -ldflags "-s -w -X github.com/bstee615/rrun/cmd.version=%{version}" \
    -o rrun .

%install
install -Dm755 rrun         %{buildroot}%{_bindir}/rrun
install -Dm644 README.md    %{buildroot}%{_docdir}/rrun/README.md
install -Dm644 CHANGELOG.md %{buildroot}%{_docdir}/rrun/CHANGELOG.md
install -Dm644 LICENSE      %{buildroot}%{_licensedir}/rrun/LICENSE

%files
%license LICENSE
%doc README.md CHANGELOG.md
%{_bindir}/rrun

%changelog
* Sun Mar 22 2026 bstee615 <benjaminjsteenhoek@gmail.com> - 0.1.0-1
- Initial package
