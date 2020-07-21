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

## Conclusion

That's it! This project can be difficult to jump into, but we do appreciate collaboration and open communication. We've
adapted these contributing guidelines from [Synapse](https://github.com/matrix-org/synapse/blob/master/CONTRIBUTING.md)
because we believe in Matrix's mission - we hope you do too and welcome you to our project!
