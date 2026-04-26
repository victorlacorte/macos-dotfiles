return {
  cmd = { 'racket', '--lib', 'racket-langserver' },
  filetypes = { 'racket', 'scheme' },
  -- root_dir = function(fname)
  --   return vim.fs.root(fname, {
  --     'info.rkt',
  --     'main.rkt',
  --     '.git',
  --   })
  -- end,
}
