#!/bin/sh

rm -f $HOME/.config/nvim/*
stow -v -d nvim-0.11 -t $HOME/.config/nvim nvim
