import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { FaHome } from "react-icons/fa";
import { api, apiJson } from "../api/client";
import type { LeaderboardRow } from "../types/leaderboard";

interface RowForm {
    name: string;
    snatch: string;
    cleanjerk: string;
}

const emptyForm: RowForm = { name: "", snatch: "", cleanjerk: "" };

const Admin: React.FC = () => {
    const [authenticated, setAuthenticated] = useState(false);
    const [checkingAuth, setCheckingAuth] = useState(true);
    const [rows, setRows] = useState<LeaderboardRow[]>([]);
    const [form, setForm] = useState<RowForm>(emptyForm);
    const [editingId, setEditingId] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const checkAuth = async () => {
            try {
                const response = await api("/api/v1/auth/check");

                if (!response.ok) {
                    throw new Error("unauthorized");
                }

                setAuthenticated(true);
            } catch (err) {
                console.error(err);
                // api() already redirected if necessary
            } finally {
                setCheckingAuth(false);
            }
        };

        checkAuth();
    }, []);

    useEffect(() => {
        const getRows = async () => {
            try {
                const rows = await apiJson<LeaderboardRow[]>("/api/v1/leaderboard");
                setRows(rows);
            } catch (error) {
                console.error(error);
            }
        };

        getRows();
    }, []);

    const setField = (field: keyof RowForm) => (value: string) =>
        setForm((prev) => ({ ...prev, [field]: value }));

    const startEdit = (row: LeaderboardRow) => {
        setEditingId(row.id);
        setForm({
            name: row.name,
            snatch: String(row.snatch),
            cleanjerk: String(row.cleanjerk),
        });
        setError(null);
    };

    const resetForm = () => {
        setForm(emptyForm);
        setEditingId(null);
        setError(null);
    };

    const submit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        const body = JSON.stringify({
            name: form.name,
            snatch: Number(form.snatch),
            cleanjerk: Number(form.cleanjerk),
        });

        try {
            if (editingId) {
                await api(`/api/v1/leaderboard/${editingId}`, {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body,
                });
            } else {
                await api("/api/v1/leaderboard", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body,
                });
            }

            const rows = await apiJson<LeaderboardRow[]>("/api/v1/leaderboard");
            setRows(rows);
            resetForm();
        } catch (err) {
            console.error(err);
            setError(editingId ? "Failed to update row." : "Failed to add row.");
        }
    };

    const deleteRow = async (id: string) => {
        setError(null);
        try {
            await api(`/api/v1/leaderboard/${id}`, { method: "DELETE" });
            setRows((prev) => prev.filter((row) => row.id !== id));
        } catch (err) {
            console.error(err);
            setError("Failed to delete row.");
        }
    };

    const inputClass =
        "input input-bordered w-full";

    if (checkingAuth) {
        return (
            <div className="h-dvh flex items-center justify-center">
                <span className="loading loading-spinner loading-lg" />
            </div>
        );
    }

    if (!authenticated) {
        return null;
    }

    return (
        <>
            <header className="sticky top-0 py-8 z-50 shadow text-center text-2xl md:text-4xl bg-base-100">
                Weightlifting Leaderboard
            </header>

            <div className="card text-neutral-content w-full min-w-[80dvw] md:max-w-[calc(100dvw-64px)] mx-auto">
                <div className="card-body items-center text-center">
                    <form
                        onSubmit={submit}
                        className="flex flex-col md:flex-row gap-2 w-full md:max-w-[60%]"
                    >
                        <input
                            type="text"
                            placeholder="Name"
                            className={inputClass}
                            value={form.name}
                            onChange={(e) => setField("name")(e.target.value)}
                            required
                        />
                        <input
                            type="number"
                            placeholder="Snatch"
                            className={inputClass}
                            value={form.snatch}
                            onChange={(e) => setField("snatch")(e.target.value)}
                            min={1}
                            required
                        />
                        <input
                            type="number"
                            placeholder="Clean & Jerk"
                            className={inputClass}
                            value={form.cleanjerk}
                            onChange={(e) => setField("cleanjerk")(e.target.value)}
                            min={1}
                            required
                        />
                        <button type="submit" className="btn btn-primary">
                            {editingId ? "Update" : "Add"}
                        </button>
                        {editingId && (
                            <button
                                type="button"
                                onClick={resetForm}
                                className="btn btn-ghost"
                            >
                                Cancel
                            </button>
                        )}
                    </form>

                    {error && <div className="text-error">{error}</div>}

                    <div className="overflow-x-auto w-full rounded-box border border-base-content/5 bg-base-100 max-h-[75dvh]">
                        <table className="table table-zebra table-pin-rows text-center">
                            <thead className="font-bold text-md md:text-2xl">
                                <tr>
                                    <th>Name</th>
                                    <th>Snatch</th>
                                    <th>Clean & Jerk</th>
                                    <th>Total</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody className="text-sm md:text-xl">
                                {rows.map((row) => (
                                    <tr key={row.id}>
                                        <td>{row.name}</td>
                                        <td>{row.snatch}</td>
                                        <td>{row.cleanjerk}</td>
                                        <td>{row.total}</td>
                                        <td>
                                            <div className="flex items-center justify-center gap-2">
                                                <button
                                                    className="btn btn-sm btn-outline"
                                                    onClick={() => startEdit(row)}
                                                >
                                                    Edit
                                                </button>
                                                <button
                                                    className="btn btn-sm btn-error btn-outline"
                                                    onClick={() => {
                                                        if (
                                                            confirm(
                                                                `Delete ${row.name}?`
                                                            )
                                                        ) {
                                                            deleteRow(row.id);
                                                        }
                                                    }}
                                                >
                                                    Delete
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
            <div className="fixed bottom-6 right-6">
                <Link to="/">
                    <button className="btn btn-lg btn-soft btn-accent btn-circle">
                        <FaHome />
                    </button>
                </Link>
            </div>
        </>
    );
};

export default Admin;
