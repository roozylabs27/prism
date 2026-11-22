import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
import { toast } from 'sonner';
import { SecretReveal } from './SecretReveal';

describe('SecretReveal', () => {
  it('shows the secret once with a warning and copies it to the clipboard', async () => {
    const user = userEvent.setup();
    render(<SecretReveal label="Gateway Secret Key" secret="prism_sk_abc123" />);

    expect(screen.getByText('prism_sk_abc123')).toBeInTheDocument();
    expect(screen.getByText(/will not be able to view this/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /copy key/i }));

    await waitFor(() => expect(screen.getByRole('button', { name: /copied/i })).toBeInTheDocument());
    expect(await navigator.clipboard.readText()).toBe('prism_sk_abc123');
    expect(toast.success).toHaveBeenCalledWith('Copied to clipboard.');
  });

  it('tells the user when copying is not possible', async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValueOnce(new Error('denied'));
    render(<SecretReveal label="Key" secret="s3cret" />);

    await user.click(screen.getByRole('button', { name: /copy key/i }));

    expect(writeText).toHaveBeenCalledWith('s3cret');
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(screen.getByRole('button', { name: /copy key/i })).toBeInTheDocument();
  });

  it('supports custom copyLabel and warningText for invitation links (DEF-006)', () => {
    render(
      <SecretReveal
        label="Invitation link"
        secret="https://app.example/accept-invite?token=abc"
        copyLabel="Copy Link"
        warningText="Share this invitation link securely with the invitee."
      />
    );

    expect(screen.getByRole('button', { name: /copy link/i })).toBeInTheDocument();
    expect(screen.getByText('Share this invitation link securely with the invitee.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /copy key/i })).not.toBeInTheDocument();
  });
});
