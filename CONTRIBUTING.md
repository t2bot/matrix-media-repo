# Contributing guidelines

Everyone is welcome to contribute code to this project, provided that they are willing to license their contributions 
under the same license as the project itself. We follow a simple 'inbound=outbound' model for contributions: the act 
of submitting an 'inbound' contribution means that the contributor agrees to license the code under the same terms as 
the project's overall 'outbound' license - in our case, this is the MIT license (see [LICENSE](LICENSE)).

## How to contribute

The preferred and easiest way to contribute changes is to fork it on GitHub, and then 
[create a pull request](https://help.github.com/articles/using-pull-requests/) to ask us to pull your changes into our repo.

We use several CI systems for testing PRs and the project in general. After opening your pull request, the build status
will be shown on GitHub. Please ensure your PR passes the builds before asking for review.

This project does not currently have unit or integration tests, though it is expected that your changes work. Please test
them locally and provide a detailed description on how they are supposed to work.

## Code style

This project doesn't yet have a linter because GoLand's default formatting rules seem good enough. If your code looks
sensible and roughly in the same shape as the code surrounding it, it will be fine.

## Changelog

Please document relevant changes in the [CHANGELOG.md](CHANGELOG.md) file. We use keep-a-changelog's format, so some
headers may need to be created.

## Sign off

In order to have a concrete record that your contribution is intentional and you agree to license it under the same 
terms as the project's license, we've adopted the same lightweight approach that the Linux Kernel
[submitting patches process](
https://www.kernel.org/doc/html/latest/process/submitting-patches.html#sign-your-work-the-developer-s-certificate-of-origin>),
[Docker](https://github.com/docker/docker/blob/master/CONTRIBUTING.md), and many other projects use: the DCO (Developer 
Certificate of Origin: http://developercertificate.org/). This is a simple declaration that you wrote the contribution 
or otherwise have the right to contribute it to us:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

If you agree to this for your contribution, then all that's needed is to
include the line in your commit or pull request comment:

```
Signed-off-by: Your Name <your@email.example.org>
```

We accept contributions under a legally identifiable name, such as your name on government documentation or common-law 
names (names claimed by legitimate usage or repute). Unfortunately, we cannot accept anonymous contributions at this time.

Git allows you to add this signoff automatically when using the `-s` flag to `git commit`, which uses the name and email 
set in your `user.name` and `user.email` git configs.

## Conclusion

That's it! This project can be difficult to jump into, but we do appreciate collaboration and open communication. We've
adapted these contributing guidelines from [Synapse](https://github.com/matrix-org/synapse/blob/master/CONTRIBUTING.md)
because we believe in Matrix's mission - we hope you do too and welcome you to our project!
