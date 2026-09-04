import { expect, request, test } from '@playwright/test';
import { ADMIN_URL } from '../playwright.config';
import { authFile, profiles, resolvePath, scenarios } from '../lib/fixtures';

// The heart of the suite: every kind=api scenario of scenarios.json is fired
// AS EVERY PROFILE. Allowed profiles must succeed (2xx), every other profile
// must be REFUSED with 403 - proving both that it works and that it is only
// reachable by the legitimate users.

for (const sc of scenarios.filter((s) => s.kind === 'api')) {
  test.describe(sc.id, () => {
    for (const profile of profiles) {
      const allowed = sc.allowed.includes(profile.id);
      test(`${profile.id} is ${allowed ? 'allowed' : 'refused'}`, async () => {
        const ctx = await request.newContext({ baseURL: ADMIN_URL, storageState: authFile(profile.id) });
        const res = await ctx.fetch(resolvePath(sc.probe!.path), {
          method: sc.probe!.method,
          data: sc.probe!.body === undefined ? undefined : sc.probe!.body,
        });
        if (allowed) {
          if (sc.probe!.allowedStatus) {
            expect(sc.probe!.allowedStatus, await res.text()).toContain(res.status());
          } else {
            expect(res.status(), await res.text()).toBeLessThan(300);
          }
        } else {
          expect(res.status(), await res.text()).toBe(403);
        }
        await ctx.dispose();
      });
    }

    test('anonymous is refused', async () => {
      const ctx = await request.newContext({ baseURL: ADMIN_URL });
      const res = await ctx.fetch(resolvePath(sc.probe!.path), {
        method: sc.probe!.method,
        data: sc.probe!.body === undefined ? undefined : sc.probe!.body,
        // Not followed, on purpose. An API answers 401 and redirects nowhere,
        // so this changes nothing for one - but a page that bounces a stranger
        // to the sign-in form was being followed to the form's own 200, and a
        // refusal that reads as a success is the one thing this test exists to
        // catch.
        maxRedirects: 0,
      });
      // 401 unless the scenario says otherwise: a consent PAGE sends a person
      // to the sign-in form, and asserting 401 there would be asserting that
      // it is an API.
      if (sc.probe!.anonymousStatus) {
        expect(sc.probe!.anonymousStatus, await res.text()).toContain(res.status());
      } else {
        expect(res.status(), await res.text()).toBe(401);
      }
      await ctx.dispose();
    });
  });
}
